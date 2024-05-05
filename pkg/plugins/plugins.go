package plugins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	hjson "github.com/hjson/hjson-go/v4"
	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
)

var (
	ErrBreakProcessing = errors.New("processing stopped")
)

// Plugin is a DNS plugin
type Plugin interface {
	Name() string

	// PrintHelp prints the configuration help for the plugin.
	PrintHelp(out io.Writer)

	// Configure the plugin.
	Configure(ctx context.Context, config map[string]interface{}) error
}

type QueryPlugin interface {
	Plugin

	// Process a Query before it is sent to the upstream server.
	Query(ctx context.Context, msg *dns.Msg) error
}

type ResponsePlugin interface {
	Plugin

	// Process a Response from the upstream server.
	Response(ctx context.Context, msg *dns.Msg) error
}

type Handler interface {
	Handle(ctx context.Context, msg *dns.Msg) (*dns.Msg, error)
}
type HandlerFunc func(ctx context.Context, msg *dns.Msg) (*dns.Msg, error)

func (f HandlerFunc) Handle(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	return f(ctx, msg)
}

// ProtocolPlugin is a plugin that handles DNS protocol specific logic.
type ProtocolServerPlugin interface {
	ResponsePlugin

	// Start the protocol plugin.
	StartServer(ctx context.Context, handler Handler) error

	// Stop the protocol plugin.
	StopServer(ctx context.Context) error
}

type ProtocolClientPlugin interface {
	QueryPlugin

	// Start the protocol plugin.
	StartClient(ctx context.Context, handler Handler) error

	// Stop the protocol plugin.
	StopClient(ctx context.Context) error
}

type ReconfigurablePlugin interface {
	Plugin

	// Reconfigure the plugin.
	Reconfigure(ctx context.Context, config map[string]interface{}) error
}

var (
	plugins         = []Plugin{}
	queryPlugins    = []QueryPlugin{}
	responsePlugins = []ResponsePlugin{}
	serverPlugins   = []ProtocolServerPlugin{}
	clientPlugins   = []ProtocolClientPlugin{}
)

// RegisterPlugin registers a plugin
func RegisterPlugin(plugin Plugin) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	log.Debug().Str("plugin", plugin.Name()).Msg("registering")
	if clientPlugin, ok := plugin.(ProtocolClientPlugin); ok {
		clientPlugins = append(clientPlugins, clientPlugin)
		orderPlugins(clientPlugins)
	}
	if serverPlugin, ok := plugin.(ProtocolServerPlugin); ok {
		serverPlugins = append(serverPlugins, serverPlugin)
		orderPlugins(queryPlugins)
	}
	if queryPlugin, ok := plugin.(QueryPlugin); ok {
		queryPlugins = append(queryPlugins, queryPlugin)
		orderPlugins(queryPlugins)
	}
	if responsePlugin, ok := plugin.(ResponsePlugin); ok {
		responsePlugins = append(responsePlugins, responsePlugin)
		orderPlugins(responsePlugins)
		slices.Reverse(responsePlugins)
	}
	plugins = append(plugins, plugin)
	orderPlugins(plugins)
}

func UnmarshalConfiguration(config map[string]interface{}, v interface{}) error {
	conf, err := hjson.Marshal(config)
	if err != nil {
		return err
	}
	return hjson.Unmarshal(conf, v)
}

func RunForAllPlugins(f func(p Plugin) error) error {
	var err error
	for _, p := range plugins {
		if err = f(p); err != nil {
			return err
		}
	}
	return err
}

// GetPlugins returns all of the registered plugins
func GetPlugins() []Plugin {
	return plugins
}

func GetClientPlugins() []ProtocolClientPlugin {
	return clientPlugins
}

func GetServerPlugins() []ProtocolServerPlugin {
	return serverPlugins
}

func GetQueryPlugins() []QueryPlugin {
	return queryPlugins
}

func GetResponsePlugins() []ResponsePlugin {
	return responsePlugins
}

func PrintPlugins[P Plugin](out io.Writer) {
	t := strings.Split(fmt.Sprintf("%T", new(P)), ".")
	fmt.Fprintf(out, "Available plugins for: %v\n", t[1])
	for _, p := range plugins {
		if _, ok := p.(P); ok {
			fmt.Fprintf(out, "%v\n", p.Name())
		}
	}
}

// Context Metadata

func ResponseMetadata(ctx context.Context) map[string]interface{} {
	val := ctx.Value(responseMetadataKey)
	if val == nil {
		return nil
	}
	return val.(map[string]interface{})
}
func QueryMetadata(ctx context.Context) map[string]interface{} {
	val := ctx.Value(queryMetadataKey)
	if val == nil {
		return nil
	}
	return val.(map[string]interface{})
}

type metadataKeyType string

const (
	responseMetadataKey = metadataKeyType("responseMetadata")
	queryMetadataKey    = metadataKeyType("queryMetadata")
)

func CreateNewHandlerCtx() context.Context {
	return ResponseCtx(QueryCtx(context.Background()))
}

func ResponseCtx(ctx context.Context) context.Context {
	metadata := make(map[string]interface{})
	return context.WithValue(ctx, responseMetadataKey, metadata)
}
func QueryCtx(ctx context.Context) context.Context {
	metadata := make(map[string]interface{})
	return context.WithValue(ctx, queryMetadataKey, metadata)
}
