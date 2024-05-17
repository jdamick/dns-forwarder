package plugins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/Eun/go-convert"
	"github.com/miekg/dns"
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
	err := applyDefaults(v)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	err = toml.NewEncoder(buf).Encode(config)
	if err != nil {
		return err
	}
	_, err = toml.NewDecoder(buf).Decode(v)
	return err
}

// Add a "comment" tag to the plugin configuration struct to provide help
func PrintPluginHelp(pluginName string, config interface{}, out io.Writer) {
	b := strings.Builder{}
	for i := 0; i < 50; i++ {
		b.WriteRune('-')
	}
	b.WriteString("\n")
	out.Write([]byte(b.String()))

	out.Write([]byte("[" + pluginName + "]\n"))
	if config != nil {
		el := reflect.TypeOf(config).Elem()
		tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
		for i := el.NumField() - 1; i >= 0; i-- {
			field := el.Field(i)
			if !field.IsExported() {
				continue
			}
			defaultVal := field.Tag.Get("default")
			if defaultVal == "" {
				defaultVal = fmt.Sprintf("%v", reflect.ValueOf(config).Elem().FieldByName(field.Name))
			}
			fieldNameType := field.Name + "=(" + field.Type.String() + ")"
			tw.Write([]byte(fieldNameType + "\t# " + field.Tag.Get("comment") + "\t(default: " + defaultVal + ")\n"))
		}
		tw.Flush()
	}
	out.Write([]byte(b.String()))
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

// add custom time duration converter since it doesnt exist in the std recipes
var converter = convert.New(convert.Options{
	Recipes: convert.MustMakeRecipes(
		func(_ convert.Converter, in string, out *time.Duration) (err error) {
			*out, err = time.ParseDuration(in)
			return
		},
	),
})

// Use the struct tag 'default' to set the default value for a field.
func applyDefaults(v interface{}) error {
	el := reflect.TypeOf(v).Elem()
	for i := el.NumField() - 1; i >= 0; i-- {
		field := el.Field(i)
		if val := field.Tag.Get("default"); val != "" {
			if err := converter.Convert(val, reflect.ValueOf(v).Elem().Field(i).Addr().Interface()); err != nil {
				return err
			}
		}
	}
	return nil
}
