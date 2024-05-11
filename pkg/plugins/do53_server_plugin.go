package plugins

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/miekg/dns"
	ants "github.com/panjf2000/ants/v2"
	log "github.com/rs/zerolog/log"
)

type DO53ServerPlugin struct {
	config    DO53ServerPluginConfig
	pool      *ants.MultiPoolWithFunc
	tcpServer *dns.Server
	udpServer *dns.Server
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&DO53ServerPlugin{})
}

func (d *DO53ServerPlugin) Name() string {
	return "dns"
}

// PrintHelp prints the configuration help for the plugin.
func (d *DO53ServerPlugin) PrintHelp(out io.Writer) {
	PrintPluginHelp(d.Name(), &d.config, out)
}

type DO53ServerPluginConfig struct {
	Listen   string `json:"listen" comment:"Listen Address and Port"`
	PoolSize int    `json:"poolSize" comment:"Worker Pool Size"`
}

const (
	defaultDo53PoolSize = 10
)

// Configure the plugin.
func (d *DO53ServerPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	log.Debug().Any("config", config).Msg("DO53ServerPluginConfig.Configure")
	d.config.PoolSize = defaultDo53PoolSize
	if err := UnmarshalConfiguration(config, &d.config); err != nil {
		return err
	}
	log.Debug().Msgf("DO53ServerPluginConfig: %#v", d.config)
	return nil
}

type reqResp struct {
	req  *dns.Msg
	resp dns.ResponseWriter
}
type responseKeyType string

var (
	responseWriterKey = responseKeyType("responseWriter")
	responseWritten   = "responseWritten"
)

// Start the protocol plugin.
func (d *DO53ServerPlugin) StartServer(sctx context.Context, handler Handler) error {
	log.Info().Msg("Starting DO53 Servers")
	p, err := ants.NewMultiPoolWithFunc(10, d.config.PoolSize, func(input interface{}) {
		r := input.(*reqResp)
		qctx := context.WithValue(CreateNewHandlerCtx(), responseWriterKey, r.resp)

		// todo make part of CreateNewHandlerCtx?
		QueryMetadata(qctx)["LocalAddr"] = r.resp.LocalAddr()
		QueryMetadata(qctx)["RemoteAddr"] = r.resp.RemoteAddr()

		ResponseMetadata(qctx)[responseWritten] = false
		handler.Handle(qctx, r.req)
		if !ResponseMetadata(qctx)[responseWritten].(bool) {
			d.Response(qctx, synthesizeErrorResponse(r.req))
		}

	}, ants.LeastTasks, ants.WithPreAlloc(true))
	if err != nil {
		return err
	}
	d.pool = p

	// start listeners
	tcpSrvr, err := d.ListenTCP()
	if err != nil {
		d.pool.ReleaseTimeout(1 * time.Millisecond)
		return err
	}
	d.tcpServer = tcpSrvr
	log.Info().Msgf("Started DO53 TCP Server on %s", d.config.Listen)

	udpSrvr, err := d.ListenUDP()
	if err != nil {
		tcpSrvr.Shutdown()
		d.pool.ReleaseTimeout(1 * time.Millisecond)
		return err
	}
	d.udpServer = udpSrvr
	log.Info().Msgf("Started DO53 UDP Server on %s", d.config.Listen)

	return nil
}

// Stop the protocol plugin.
func (d *DO53ServerPlugin) StopServer(ctx context.Context) error {
	d.tcpServer.Shutdown()
	d.udpServer.Shutdown()
	d.pool.ReleaseTimeout(1 * time.Millisecond)
	return nil
}

func (d *DO53ServerPlugin) Response(ctx context.Context, msg *dns.Msg) error {
	log.Debug().Msgf("Response: %v", msg)
	// get the response key and writer and write to it.
	ResponseMetadata(ctx)[responseWritten] = true
	return ctx.Value(responseWriterKey).(dns.ResponseWriter).WriteMsg(msg)
}

func (d *DO53ServerPlugin) handleIncoming(w dns.ResponseWriter, req *dns.Msg) {
	d.pool.Invoke(&reqResp{req: req, resp: w})
}

func (d *DO53ServerPlugin) ListenTCP() (*dns.Server, error) {
	waitLock := sync.Mutex{}
	//addr := ":0"
	proto := "tcp"
	// if proto == "tcp6" {
	// 	addr = "[::]:0"
	// }
	server := &dns.Server{
		Addr:              d.config.Listen,
		Net:               proto,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		NotifyStartedFunc: waitLock.Unlock,
		ReusePort:         true,
		ReuseAddr:         true,
		Handler:           dns.HandlerFunc(d.handleIncoming)}
	waitLock.Lock()

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start TCP server")
		}
	}()
	waitLock.Lock()
	return server, nil
}

func (d *DO53ServerPlugin) ListenUDP() (*dns.Server, error) {
	waitLock := sync.Mutex{}

	//	net.ResolveUDPAddr("udp", d.config.Listen)
	// addr := ":0"
	proto := "udp"
	// if proto == "udp6" {
	// 	addr = "[::]:0"
	// }
	server := &dns.Server{
		Addr:              d.config.Listen,
		Net:               proto,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		NotifyStartedFunc: waitLock.Unlock,
		ReusePort:         true,
		ReuseAddr:         true,
		UDPSize:           4096,
		Handler:           dns.HandlerFunc(d.handleIncoming)}
	waitLock.Lock()

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start UDP server")
		}
	}()
	waitLock.Lock()
	return server, nil
}

func synthesizeErrorResponse(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetRcode(req, dns.RcodeServerFailure)
	return resp
}
