package plugins

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"time"

	iradix "github.com/hashicorp/go-immutable-radix/v2"
	"github.com/miekg/dns"
	log "github.com/rs/zerolog/log"
)

type DO53ClientPlugin struct {
	//config  DO53ClientPluginConfig
	handler Handler
	clients *iradix.Tree[*do53client]
}

const (
	defaultDO53Timeout = time.Second * 2
)

type DO53ClientPluginConfig struct {
	AlwaysRetryOverTcp bool     `json:"alwaysRetryOverTCP" comment:"Always Retry a Failed UDP Query over TCP"`
	Upstream           []string `json:"upstream" comment:"Address and Port of upstream nameserver"`
	Timeout            string   `json:"timeout" comment:"Timeout duration"`
	timeoutDuration    time.Duration
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&DO53ClientPlugin{clients: iradix.New[*do53client]()})
}

func (d *DO53ClientPlugin) Name() string {
	return "dnsclient"
}

// PrintHelp prints the configuration help for the plugin.
func (d *DO53ClientPlugin) PrintHelp(out io.Writer) {
	out.Write([]byte("DO53ClientPlugin\n"))
}

// Configure the plugin.
func (d *DO53ClientPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	log.Debug().Any("config", config).Msg("DO53ClientPlugin.Configure")

	// get each domain configured
	for domain, cfg := range config {
		if cfg == nil {
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
		if client.config.timeoutDuration == 0 {
			client.config.timeoutDuration = defaultDO53Timeout
		}

		revDomain := ReverseString(dns.CanonicalName(domain))
		var ok bool
		d.clients, _, ok = d.clients.Insert([]byte(revDomain), client)
		if !ok {
			it := d.clients.Root().Iterator()
			for k, client, ok := it.Next(); ok; k, client, ok = it.Next() {
				log.Debug().Str("k", string(k)).Msgf("tree: %v", client)
			}
			//	return fmt.Errorf("failed to insert domain: %v as %v", domain, revDomain)
		}
		log.Debug().Msgf("DO53Client: %#v", client.config)
	}
	log.Debug().Msgf("DO53ClientPluginConfig")
	return nil
}

// Start the protocol plugin.
func (d *DO53ClientPlugin) StartClient(ctx context.Context, handler Handler) error {
	log.Info().Msg("Starting DO53 Client")
	d.handler = handler
	it := d.clients.Root().Iterator()
	for k, client, ok := it.Next(); ok; k, client, ok = it.Next() {
		log.Debug().Str("domain", string(k)).Msg("Starting DO53 Client")
		if err := client.StartClient(ctx, handler); err != nil {
			return err
		}
	}
	return nil
}

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
	revDomain := ReverseString(dns.CanonicalName(qname))
	if _, client, ok := d.clients.Root().LongestPrefix([]byte(revDomain)); ok {
		return client.Query(ctx, msg)
	}
	return nil
}

type do53client struct {
	domain  string
	config  DO53ClientPluginConfig
	handler Handler
}

// Start the protocol plugin.
func (d *do53client) StartClient(ctx context.Context, handler Handler) error {
	log.Info().Msg("Starting DO53 Client")
	d.handler = handler
	return nil
}

// Stop the protocol plugin.
func (d *do53client) StopClient(ctx context.Context) error {
	return nil
}

func (d *do53client) Query(ctx context.Context, msg *dns.Msg) error {
	log.Debug().Msgf("DO53ClientPlugin.Query: %v\n", msg)
	msg.Compress = true
	q, err := msg.Pack()
	if err != nil {
		return err
	}
	// todo upstream array..
	var resp []byte
	resp, _ /*rtt*/, err = udpQuery(d.config.Upstream[0], d.config.timeoutDuration, q)
	log.Debug().Msgf("DO53ClientPlugin resp: %v err: %v\n", resp, err)

	respMsg := &dns.Msg{}
	respMsg.Compress = true
	if err == nil {
		err = respMsg.Unpack(resp)
	}
	log.Debug().Msgf("DO53ClientPlugin resp2: %v\n", respMsg)
	log.Debug().Msgf("respMsg.Truncated: %v err: %v\n", respMsg.Truncated, err)
	if respMsg.Truncated || (d.config.AlwaysRetryOverTcp && err != nil) {
		log.Debug().Msgf("Trying over TCP\n")
		// is resp is truncated or some udp error, try tcp..
		resp, _ /*rtt*/, err = tcpQuery(d.config.Upstream[0], d.config.timeoutDuration, q)
		if err != nil {
			return err
		}
		if err = respMsg.Unpack(resp); err != nil {
			return err
		}
	}
	if err != nil {
		return err
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

func udpQuery(serverAddress string, timeout time.Duration, query []byte) ([]byte, time.Duration, error) {
	//defer SimpleScopeTiming("udpExchange3")()
	var rtt time.Duration
	packet := []byte{}
	udpAddr, err := net.ResolveUDPAddr(udpProto, serverAddress)
	if err != nil {
		return packet, rtt, err
	}
	upstreamAddr := udpAddr

	now := time.Now()
	lAddr, err := net.ResolveUDPAddr(udpProto, ":0")
	if err != nil {
		return packet, rtt, err
	}

	conn, err := net.ListenUDP(udpProto, lAddr)

	if err != nil {
		return packet, rtt, err
	}
	defer conn.Close()
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
