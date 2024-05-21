package plugins

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"reflect"
	"runtime"
	"time"

	iradix "github.com/hashicorp/go-immutable-radix/v2"
	utils "github.com/jdamick/dns-forwarder/pkg/utils"
	"github.com/miekg/dns"
	log "github.com/rs/zerolog/log"
)

type DO53ClientPlugin struct {
	baseConfig DO53ClientPluginConfig
	handler    Handler
	clients    *iradix.Tree[*do53client]
}

type DO53ClientPluginConfig struct {
	AlwaysRetryOverTcp bool     `toml:"alwaysRetryOverTCP" comment:"Always Retry a Failed UDP Query over TCP" default:"true"`
	Upstream           []string `toml:"upstream" comment:"Address and Port of upstream nameserver"`
	UdpConnPoolSize    int      `toml:"udpConnectionPoolSize" comment:"UDP Connection Pool Size" default:"8000"`
	Timeout            string   `toml:"timeout" comment:"Timeout duration" default:"2s"`
	timeoutDuration    time.Duration
	//UDPSize            uint16   `toml:"udpsize" comment:"Max size of UDP response" default:"1232"`
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&DO53ClientPlugin{
		clients: iradix.New[*do53client](),
	})
}

func (d *DO53ClientPlugin) Name() string {
	return "dnsclient"
}

// PrintHelp prints the configuration help for the plugin.
func (d *DO53ClientPlugin) PrintHelp(out io.Writer) {
	PrintPluginHelp(d.Name(), &DO53ClientPluginConfig{}, out)
	PrintPluginHelp(d.Name()+".\"<optional domain specific>\"", &DO53ClientPluginConfig{}, out)
}

// Configure the plugin.
func (d *DO53ClientPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	log.Debug().Any("config", config).Msg("DO53ClientPlugin.Configure")

	if err := UnmarshalConfiguration(config, &d.baseConfig); err != nil {
		return err
	}
	log.Debug().Any("base config", d.baseConfig).Msg("DO53ClientPlugin.Configure")

	// get each domain configured
	for domain, cfg := range config {
		if cfg == nil || reflect.TypeOf(cfg) != reflect.TypeOf(map[string]interface{}{}) {
			continue
		}

		log.Debug().Str("domain", domain).Any("config", cfg).Msg("DO53ClientPlugin.Configure")

		client := &do53client{domain: domain}

		if err := UnmarshalConfiguration(cfg.(map[string]interface{}), &client.config); err != nil {
			return err
		}
		if client.config.Timeout != "" {
			var err error
			client.config.timeoutDuration, err = time.ParseDuration(client.config.Timeout)
			if err != nil {
				return err
			}
		}

		revDomain := utils.ReverseString(dns.CanonicalName(domain))
		var ok bool
		d.clients, _, ok = d.clients.Insert([]byte(revDomain), client)
		if !ok {
			it := d.clients.Root().Iterator()
			for k, client, ok := it.Next(); ok; k, client, ok = it.Next() {
				log.Debug().Str("k", string(k)).Msgf("tree: %v", client)
			}
		}
		log.Debug().Msgf("DO53Client: %#v", client.config)
	}
	log.Debug().Msgf("DO53ClientPluginConfig")
	return nil
}

type udpConnPool = *utils.RingBuffer[*net.UDPConn]
type tcpConnPool = *utils.RingBuffer[*net.TCPConn]

// Start the protocol plugin.
func (d *DO53ClientPlugin) StartClient(ctx context.Context, handler Handler) error {
	log.Info().Msg("Starting DO53 Client")

	// connectin pooling
	udpPool := utils.NewRingBuffer[*net.UDPConn](8_000)
	tries := 0
ConnFill:
	for i := udpPool.Len(); i < udpPool.Cap(); i++ {
		conn := createUDPConn()
		if conn == nil {
			if tries < 3 {
				tries++
				runtime.Gosched()
				continue ConnFill
			}
			continue
		}
		udpPool.Enqueue(conn)
	}
	if !udpPool.Full() {
		log.Error().Uint64("udpPool", udpPool.Len()).Msg("failed to fill up UDP connection pool")
	}

	d.handler = handler
	it := d.clients.Root().Iterator()
	for k, client, ok := it.Next(); ok; k, client, ok = it.Next() {
		log.Debug().Str("domain", string(k)).Msg("Starting DO53 Client")
		if err := client.StartClient(ctx, udpPool, nil, handler); err != nil {
			return err
		}
	}
	return nil
}

func createUDPConn() *net.UDPConn {
	lAddr, err := net.ResolveUDPAddr(udpProto, ":0")
	if err != nil {
		log.Error().Err(err).Msg("ResolveUDPAddr failed")
	}
	conn, err := net.ListenUDP(udpProto, lAddr)
	if err != nil {
		log.Error().Err(err).Msg("ListenUDP failed")
	}
	return conn
}

/*
func createTCPConn() *net.TCPConn {
	lAddr, err := net.ResolveTCPAddr(tcpProto, ":0")
	if err != nil {
		log.Error().Err(err).Msg("ResolveTCPAddr failed")
	}
	conn, err := net.ListenTCP(tcpProto, lAddr)
	if err != nil {
		log.Error().Err(err).Msg("ListenTCP failed")
	}
	return conn
}
*/
// Stop the protocol plugin.
func (d *DO53ClientPlugin) StopClient(ctx context.Context) error {
	it := d.clients.Root().Iterator()
	for k, client, ok := it.Next(); ok; k, client, ok = it.Next() {
		log.Debug().Str("domain", string(k)).Msg("Stopping DO53 Client")
		if err := client.StopClient(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (d *DO53ClientPlugin) Query(ctx context.Context, msg *dns.Msg) error {
	if len(msg.Question) == 0 {
		return nil
	}
	qname := msg.Question[0].Name
	revDomain := utils.ReverseString(dns.CanonicalName(qname))
	if _, client, ok := d.clients.Root().LongestPrefix([]byte(revDomain)); ok {
		return client.Query(ctx, msg)
	}
	return nil
}

func (d *DO53ClientPluginConfig) pickUpstream() string {
	if len(d.Upstream) == 0 {
		return ""
	}
	idx := rand.IntN(len(d.Upstream))
	return d.Upstream[idx]
}

type do53client struct {
	domain  string
	config  DO53ClientPluginConfig
	handler Handler
	udpPool udpConnPool
	tcpPool tcpConnPool
}

// Start the protocol plugin.
func (d *do53client) StartClient(ctx context.Context, udpPool udpConnPool, tcpPool tcpConnPool, handler Handler) error {
	log.Info().Msg("Starting DO53 Client")
	d.handler = handler
	d.udpPool = udpPool
	d.tcpPool = tcpPool
	return nil
}

// Stop the protocol plugin.
func (d *do53client) StopClient(ctx context.Context) error {
	return nil
}

func (d *do53client) Query(ctx context.Context, msg *dns.Msg) error {
	//log.Debug().Msgf("DO53ClientPlugin.Query: %v\n", msg)
	msg.Compress = true
	q, err := msg.Pack()
	if err != nil {
		return err
	}

	var resp []byte
	up := d.config.pickUpstream()
	log.Debug().Msgf("sending udp query to upstream: %v", up)
	c := d.udpConn()
	if c == nil {
		return fmt.Errorf("no udp connections available")
	}
	defer d.udpPool.Enqueue(c)
	resp, _ /*rtt*/, err = udpQuery(c, up, d.config.timeoutDuration, q)

	respMsg := &dns.Msg{}
	respMsg.Compress = true
	if err == nil {
		err = respMsg.Unpack(resp)
	}

	if respMsg.Truncated || (d.config.AlwaysRetryOverTcp && err != nil) {
		log.Debug().Msgf("sending tcp query to upstream: %v due to truncation? %v", up, respMsg.Truncated)
		// is resp is truncated or some udp error, try tcp..
		resp, _ /*rtt*/, err = tcpQuery(up, d.config.timeoutDuration, q)
		if err != nil {
			return fmt.Errorf("upstream: %v tcp error: %w", up, err)
		}
		if err = respMsg.Unpack(resp); err != nil {
			return fmt.Errorf("upstream: %v unpack error: %w", up, err)
		}
	}
	if err != nil {
		return fmt.Errorf("upstream: %v %w", up, err)
	}
	// send back through the handler..
	if d.handler != nil {
		_, err = d.handler.Handle(ctx, respMsg)
	}
	return err
}

const (
	maxUDPPacketSize = 4096
	udpProto         = "udp"
)

func (d *do53client) udpConn() *net.UDPConn {
	conn, ok := d.udpPool.Dequeue()
	if !ok {
		return nil
	}
	return conn
}

func udpQuery(conn *net.UDPConn, serverAddress string, timeout time.Duration, query []byte) ([]byte, time.Duration, error) {
	//defer SimpleScopeTiming("udpExchange3")()
	var rtt time.Duration
	packet := []byte{}
	udpAddr, err := net.ResolveUDPAddr(udpProto, serverAddress)
	if err != nil {
		return packet, rtt, err
	}
	upstreamAddr := udpAddr

	now := time.Now()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return packet, rtt, err
	}

	if _, err := conn.WriteToUDPAddrPort(query, upstreamAddr.AddrPort()); err != nil {
		return packet, rtt, err
	}

	packet = make([]byte, maxUDPPacketSize)
	length, _, err := conn.ReadFrom(packet)
	if err != nil {
		return packet, rtt, err
	}

	rtt = time.Since(now)
	packet = packet[:length]
	return packet, rtt, err
}

func tcpQuery(serverAddress string, timeout time.Duration, query []byte) ([]byte, time.Duration, error) {
	var rtt time.Duration
	response := []byte{}
	tcpAddr, err := net.ResolveTCPAddr("tcp", serverAddress)
	if err != nil {
		return response, rtt, err
	}
	upstreamAddr := tcpAddr
	start := time.Now()
	var conn net.Conn
	conn, err = net.DialTCP("tcp", nil, upstreamAddr)
	if err != nil {
		return response, rtt, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return response, rtt, err
	}

	// add tcp 2 byte length prefix
	bufLen := make([]byte, 2)
	binary.BigEndian.PutUint16(bufLen, uint16(len(query)))

	n, err := conn.Write(bufLen)
	if err != nil || n != len(bufLen) {
		return response, rtt, err
	}
	n, err = conn.Write(query)
	if err != nil || n != len(query) {
		return response, rtt, err
	}

	// first read the 2 byte length prefix
	bufLen[0], bufLen[1] = 0, 0
	n, err = conn.Read(bufLen)
	if err != nil || n != len(bufLen) {
		return response, rtt, err
	}
	len := binary.BigEndian.Uint16(bufLen)
	response = make([]byte, len)
	n, err = conn.Read(response)
	if err != nil || uint16(n) != len {
		return response, rtt, err
	}
	rtt = time.Since(start)

	return response, rtt, err
}
