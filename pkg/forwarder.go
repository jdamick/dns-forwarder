package dnsforwarder

import (
	"context"
	"io"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/VictoriaMetrics/metrics"
	plugins "github.com/jdamick/dns-forwarder/pkg/plugins"
	utils "github.com/jdamick/dns-forwarder/pkg/utils"
	"github.com/miekg/dns"
	log "github.com/rs/zerolog/log"
)

type Forwarder struct {
	configuredPlugins map[string]plugins.Plugin
}

func NewForwarder() *Forwarder {
	return &Forwarder{configuredPlugins: make(map[string]plugins.Plugin)}
}

func (f *Forwarder) PrintHelp(pluginName string, out io.Writer) {
	plugins.RunForAllPlugins(func(p plugins.Plugin) error {
		if p.Name() == pluginName {
			p.PrintHelp(out)
		}
		return nil
	})
}

func (f *Forwarder) Configure(conf []byte) error {
	var confMap map[string]interface{}
	err := toml.Unmarshal(conf, &confMap)
	if err != nil {
		return err
	}
	ctx := context.Background()

	err = plugins.RunForAllPlugins(func(p plugins.Plugin) error {
		if v, ok := confMap[p.Name()]; ok {
			// might be an array... todo
			nestedConf := v.(map[string]interface{})
			err = p.Configure(ctx, nestedConf)
			if err != nil {
				return err
			}
			f.configuredPlugins[p.Name()] = p
		}
		return nil
	})

	return err
}

func (f *Forwarder) ConfigureFrom(conf io.Reader) error {
	buf, err := io.ReadAll(conf)
	if err != nil {
		return err
	}
	return f.Configure(buf)
}

func (f *Forwarder) isPluginConfigured(name string) bool {
	_, ok := f.configuredPlugins[name]
	return ok
}

func (f *Forwarder) Start() error {
	if log.Debug().Enabled() && false {
		plugins.PrintPlugins[plugins.QueryPlugin](os.Stdout)
	}

	ctx := context.Background()
	var err error

	// Now Start the Client Plugins
	for _, p := range plugins.GetClientPlugins() {
		if f.isPluginConfigured(p.Name()) {
			err = p.StartClient(ctx, plugins.HandlerFunc(f.ResponseHandler))
			if err != nil {
				log.Fatal().Str("name", p.Name()).Err(err).Msg("error starting plugin")
			}
		}
	}

	// Now Start the Server Plugins
	for _, p := range plugins.GetServerPlugins() {
		if f.isPluginConfigured(p.Name()) {
			err = p.StartServer(ctx, plugins.HandlerFunc(f.QueryHandler))
			if err != nil {
				log.Fatal().Str("name", p.Name()).Err(err).Msg("error starting plugin")
			}
		}
	}
	return err
}

const (
	responseHandlerCalled = "responseHandlerCalled"
)

func (f *Forwarder) QueryHandler(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	log.Debug().Msg("QueryHandler")

	defer utils.SimpleScopeTiming("dns_query_duration")()
	metrics.GetOrCreateCounter("dns_query_inflight_count").Inc()
	defer metrics.GetOrCreateCounter("dns_query_inflight_count").Dec()

	ctx = setupHandlerCtx(ctx) // make sure the ctx is setup correctly
	plugins.ResponseMetadata(ctx)[responseHandlerCalled] = false
	for _, p := range plugins.GetQueryPlugins() {
		log.Debug().Str("name", p.Name()).Msg("QueryHandler")
		if f.isPluginConfigured(p.Name()) {
			err := p.Query(ctx, msg)
			log.Debug().Str("name", p.Name()).Err(err).Msg("QueryHandler")
			if err == plugins.ErrBreakProcessing || plugins.ResponseMetadata(ctx)[responseHandlerCalled].(bool) {
				return nil, nil
			}
			if err != nil {
				log.Error().Err(err).Msg("query processing error")
				return nil, err
			}
		}
	}
	return nil, nil
}

func (f *Forwarder) ResponseHandler(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	log.Debug().Msg("ResponseHandler")
	ctx = setupHandlerCtx(ctx) // make sure the ctx is setup correctly]
	plugins.ResponseMetadata(ctx)[responseHandlerCalled] = true
	for _, p := range plugins.GetResponsePlugins() {
		log.Debug().Str("name", p.Name()).Msg("ResponseHandler")
		if f.isPluginConfigured(p.Name()) {
			err := p.Response(ctx, msg)
			log.Debug().Str("name", p.Name()).Err(err).Msg("ResponseHandler")
			if err == plugins.ErrBreakProcessing {
				return nil, nil
			}
			if err != nil {
				log.Error().Err(err).Msg("response processing error")
				return nil, err
			}
		}
	}
	return nil, nil
}

func (f *Forwarder) Stop() {
	ctx := context.Background()

	for _, p := range plugins.GetClientPlugins() {
		if f.configuredPlugins[p.Name()] != nil {
			p.StopClient(ctx)
		}
	}

	for _, p := range plugins.GetServerPlugins() {
		if f.configuredPlugins[p.Name()] != nil {
			p.StopServer(ctx)
		}
	}
}

func setupHandlerCtx(ctx context.Context) context.Context {
	if plugins.QueryMetadata(ctx) == nil {
		ctx = plugins.QueryCtx(ctx)
	}
	if plugins.ResponseMetadata(ctx) == nil {
		ctx = plugins.ResponseCtx(ctx)
	}
	return ctx
}
