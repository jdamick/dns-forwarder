package plugins

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/miekg/dns"
	ants "github.com/panjf2000/ants/v2"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
)

type DO53GnetServerPlugin struct {
	config     DO53GnetServerPluginConfig
	udpPool    *ants.MultiPoolWithFunc
	tcpPool    *ants.MultiPoolWithFunc
	engines    []gnet.Engine
	mutex      sync.Mutex
	startMutex sync.Mutex
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&DO53GnetServerPlugin{})
}

func (d *DO53GnetServerPlugin) Name() string {
	return "gnetdns"
}

// PrintHelp prints the configuration help for the plugin.
func (d *DO53GnetServerPlugin) PrintHelp(out io.Writer) {
	PrintPluginHelp(d.Name(), &d.config, out)
}

type DO53GnetServerPluginConfig struct {
	Listen                 string `toml:"listen" comment:"Listen Address and Port"`
	PoolSizeTCP            int    `toml:"tcpPoolSize" comment:"Worker Pool Size"`
	PoolSizeUDP            int    `toml:"udpPoolSize" comment:"Worker Pool Size"`
	numEventLoopTCP        int
	numEventLoopUDP        int
	bufferSizeTCP          int
	bufferSizeUDP          int
	maxQueriesPerTCPPacket int
	tcpKeepAlive           time.Duration
	EnableLogging          bool `toml:"enableLogging" comment:"Enable Logging on gnet"`
}

const (
// defaultDo53PoolSize = 10
)

// Configure the plugin.
func (d *DO53GnetServerPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	log.Debug().Any("config", config).Msg("DO53GnetServerPlugin.Configure")

	// todo
	d.config.numEventLoopTCP = runtime.NumCPU()
	d.config.numEventLoopUDP = runtime.NumCPU()
	d.config.PoolSizeTCP = defaultDo53PoolSize
	d.config.PoolSizeUDP = defaultDo53PoolSize
	d.config.bufferSizeTCP = 10 * 1024
	d.config.bufferSizeUDP = 10 * 1024
	d.config.maxQueriesPerTCPPacket = 50
	d.config.tcpKeepAlive = 10 * time.Second
	d.config.EnableLogging = false

	if err := UnmarshalConfiguration(config, &d.config); err != nil {
		return err
	}
	log.Debug().Msgf("DO53GnetServerPlugin: %#v", d.config)
	return nil
}

type gReqResp struct {
	req        *dns.Msg
	localAddr  net.Addr
	remoteAddr net.Addr
	conn       gnet.Conn
}

// type responseKeyType string

// var (
// 	responseWriterKey = responseKeyType("responseWriter")
// 	responseWritten   = "responseWritten"
// )

// Start the protocol plugin.
func (d *DO53GnetServerPlugin) StartServer(sctx context.Context, handler Handler) error {
	log.Info().Msg("Starting DO53 Servers")

	poolJob := func(input interface{}) {
		r := input.(*gReqResp)
		qctx := context.WithValue(CreateNewHandlerCtx(), responseWriterKey, r.conn)

		// todo make part of CreateNewHandlerCtx?
		QueryMetadata(qctx)["LocalAddr"] = r.localAddr
		QueryMetadata(qctx)["RemoteAddr"] = r.remoteAddr

		ResponseMetadata(qctx)[responseWritten] = false

		handler.Handle(qctx, r.req)

		if !ResponseMetadata(qctx)[responseWritten].(bool) {
			d.Response(qctx, SynthesizeErrorResponse(r.req))
		}
	}

	udpPool, err := ants.NewMultiPoolWithFunc(10, d.config.PoolSizeUDP, poolJob, ants.LeastTasks, ants.WithPreAlloc(true))
	if err != nil {
		return err
	}
	d.udpPool = udpPool

	tcpPool, err := ants.NewMultiPoolWithFunc(10, d.config.PoolSizeTCP, poolJob, ants.LeastTasks, ants.WithPreAlloc(true))
	if err != nil {
		return err
	}
	d.tcpPool = tcpPool

	// start listeners
	err = d.ListenTCP()
	if err != nil {
		d.StopServer(sctx)
		return err
	}

	log.Info().Msgf("Started DO53 TCP Server on %s", d.config.Listen)

	err = d.ListenUDP()
	if err != nil {
		d.StopServer(sctx)
		return err
	}

	log.Info().Msgf("Started DO53 UDP Server on %s", d.config.Listen)

	return nil
}

// Stop the protocol plugin.
func (d *DO53GnetServerPlugin) StopServer(ctx context.Context) error {
	// retrieve a list of the current engines in a lock
	d.mutex.Lock()
	engines := append([]gnet.Engine{}, d.engines...)
	d.mutex.Unlock()

	// now stop them all
	for _, e := range engines {
		e.Stop(ctx)
	}

	d.udpPool.ReleaseTimeout(1 * time.Millisecond)
	d.tcpPool.ReleaseTimeout(1 * time.Millisecond)
	return nil
}

func (d *DO53GnetServerPlugin) Response(ctx context.Context, msg *dns.Msg) error {
	log.Debug().Msgf("Response: %v", msg)
	// get the response key and writer and write to it.
	ResponseMetadata(ctx)[responseWritten] = true
	msg.Compress = true
	// todo buf pool
	buf, err := msg.Pack()
	if err != nil {
		return err
	}
	n, err := ctx.Value(responseWriterKey).(gnet.Conn).Write(buf)
	if n != len(buf) {
		log.Error().Err(err).Msgf("response write error")
		return fmt.Errorf("response write error")
	}
	return err
}

// Gnet Events
func (d *DO53GnetServerPlugin) OnBoot(eng gnet.Engine) (action gnet.Action) {
	d.startMutex.Unlock()

	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.engines = append(d.engines, eng)
	return
}

func (d *DO53GnetServerPlugin) OnShutdown(eng gnet.Engine) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	for i, e := range d.engines {
		if e == eng {
			d.engines = append(d.engines[:i], d.engines[i+1:]...)
			break
		}
	}
}

func (d *DO53GnetServerPlugin) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	return
}

func (d *DO53GnetServerPlugin) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	return
}

func (d *DO53GnetServerPlugin) OnTraffic(c gnet.Conn) (action gnet.Action) {
	in := []byte{}
	var err error
	_, tcp := c.LocalAddr().(*net.TCPAddr)
	if tcp {
		// could be multiple queries in one packet
		for i := 0; i < d.config.maxQueriesPerTCPPacket; i++ {
			in, err = c.Peek(2)
			if err != nil {
				return
			}
			len := binary.BigEndian.Uint16(in)
			if c.InboundBuffered() >= int(2+len) {
				c.Discard(2)
				in, err = c.Next(int(len))
			} else {
				return
			}
		}
	} else {
		in, err = c.Next(-1)
	}

	if err != nil || len(in) == 0 {
		return
	}

	req := new(dns.Msg)
	err = req.Unpack(in)
	if err != nil {
		log.Error().Err(err).Msg("Unpack")
		return
	}

	jobParam := &gReqResp{req: req, conn: c,
		remoteAddr: DeepCopyAddr(c.RemoteAddr()),
		localAddr:  DeepCopyAddr(c.LocalAddr())}
	if tcp {
		// todo
	} else {
		d.udpPool.Invoke(jobParam)
	}

	return
}

func (d *DO53GnetServerPlugin) OnTick() (delay time.Duration, action gnet.Action) {
	return
}

func (d *DO53GnetServerPlugin) ListenTCP() error {
	//addr := ":0"
	proto := "tcp"
	// if proto == "tcp6" {
	// 	addr = "[::]:0"
	// }
	lvl := logging.FatalLevel
	if d.config.EnableLogging {
		lvl = mapCurentLogLevelToGnet()
	}

	var err error
	d.startMutex.Lock()
	defer d.startMutex.Unlock()
	go func() {
		err = gnet.Run(d, proto+"://"+d.config.Listen,
			gnet.WithLogger(&gnetLogAdapter{}),
			gnet.WithLogLevel(lvl),
			gnet.WithEdgeTriggeredIO(true),
			gnet.WithNumEventLoop(d.config.numEventLoopTCP),
			gnet.WithReusePort(true),
			gnet.WithReuseAddr(true),
			gnet.WithReadBufferCap(d.config.bufferSizeTCP),
			gnet.WithSocketRecvBuffer(d.config.bufferSizeTCP),
			gnet.WithTCPKeepAlive(d.config.tcpKeepAlive))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start TCP server")
			d.startMutex.Unlock()
		}
	}()
	d.startMutex.Lock()
	return err
}

func (d *DO53GnetServerPlugin) ListenUDP() error {

	//	net.ResolveUDPAddr("udp", d.config.Listen)
	// addr := ":0"
	proto := "udp"
	// if proto == "udp6" {
	// 	addr = "[::]:0"
	// }
	lvl := logging.FatalLevel
	if d.config.EnableLogging {
		lvl = mapCurentLogLevelToGnet()
	}

	var err error
	d.startMutex.Lock()
	defer d.startMutex.Unlock()
	go func() {
		err = gnet.Run(d, proto+"://"+d.config.Listen,
			gnet.WithLogger(&gnetLogAdapter{}),
			gnet.WithLogLevel(lvl),
			gnet.WithEdgeTriggeredIO(true),
			gnet.WithNumEventLoop(d.config.numEventLoopUDP),
			gnet.WithReusePort(true),
			gnet.WithReuseAddr(true),
			gnet.WithReadBufferCap(d.config.bufferSizeUDP),
			gnet.WithSocketRecvBuffer(d.config.bufferSizeUDP),
			gnet.WithWriteBufferCap(d.config.bufferSizeUDP),
			gnet.WithSocketSendBuffer(d.config.bufferSizeUDP))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start UDP server")
			d.startMutex.Unlock()
		}
	}()
	d.startMutex.Lock()
	log.Debug().Msg("UDP started")
	return err
}

func mapCurentLogLevelToGnet() logging.Level {
	switch zerolog.GlobalLevel() {
	case zerolog.DebugLevel:
		return logging.DebugLevel
	case zerolog.InfoLevel:
		return logging.InfoLevel
	case zerolog.WarnLevel:
		return logging.WarnLevel
	case zerolog.ErrorLevel:
		return logging.ErrorLevel
	case zerolog.FatalLevel:
		return logging.FatalLevel
	default:
		return logging.InfoLevel
	}
}

type gnetLogAdapter struct {
}

func (g *gnetLogAdapter) Debugf(format string, args ...interface{}) {
	log.Debug().Msgf(format, args...)
}

// Infof logs messages at INFO level.
func (g *gnetLogAdapter) Infof(format string, args ...interface{}) {
	log.Info().Msgf(format, args...)
}

// Warnf logs messages at WARN level.
func (g *gnetLogAdapter) Warnf(format string, args ...interface{}) {
	log.Warn().Msgf(format, args...)
}

// Errorf logs messages at ERROR level.
func (g *gnetLogAdapter) Errorf(format string, args ...interface{}) {
	log.Error().Msgf(format, args...)
}

// Fatalf logs messages at FATAL level.
func (g *gnetLogAdapter) Fatalf(format string, args ...interface{}) {
	log.Fatal().Msgf(format, args...)
}
