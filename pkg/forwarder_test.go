package dnsforwarder

import (
	"context"
	"fmt"
	"io"
	"testing"

	plugins "github.com/jdamick/dns-forwarder/pkg/plugins"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestForwarderConfigure(t *testing.T) {
	f := NewForwarder()
	assert := assert.New(t)

	assert.False(f.isPluginConfigured("test-plugin-1"))

	tp := &TestPlugin{name: "test-plugin-1"}
	plugins.RegisterPlugin(tp)
	assert.Nil(f.Configure([]byte(`["test-plugin-1"]`)))

	assert.True(f.isPluginConfigured("test-plugin-1"))
}

func TestForwarderServerPlugin(t *testing.T) {
	f := NewForwarder()
	assert := assert.New(t)

	tp := &testServerPlugin{t: t, name: "test-server-1"}
	plugins.RegisterPlugin(tp)

	serverPluginFound := false
	for _, p := range plugins.GetServerPlugins() {
		if p.Name() == "test-server-1" {
			serverPluginFound = true
			break
		}
	}
	assert.True(serverPluginFound)

	assert.NoError(f.Configure([]byte(`["test-server-1"]`)))
	assert.True(f.isPluginConfigured("test-server-1"))
	assert.NoError(f.Start())

	f.ResponseHandler(context.Background(), &dns.Msg{})

	f.Stop()

	assert.Equal(1, tp.startCalled)
	assert.Equal(1, tp.responseCalled)
	assert.Equal(1, tp.stopCalled)
}

func TestForwarderClientPlugin(t *testing.T) {
	f := NewForwarder()
	assert := assert.New(t)

	tp := &testClientPlugin{t: t, name: "test-client-1"}
	plugins.RegisterPlugin(tp)

	clientPluginFound := false
	for _, p := range plugins.GetClientPlugins() {
		if p.Name() == "test-client-1" {
			clientPluginFound = true
			break
		}
	}
	assert.True(clientPluginFound)

	assert.NoError(f.Configure([]byte(`["test-client-1"]`)))
	assert.True(f.isPluginConfigured("test-client-1"))
	assert.NoError(f.Start())

	f.QueryHandler(context.Background(), &dns.Msg{})

	f.Stop()

	assert.Equal(1, tp.startCalled, "start not called")
	assert.Equal(1, tp.queryCalled, "query not called")
	assert.Equal(1, tp.stopCalled, "stop not called")
}

// Mock Plugins
///////////////

// Server Plugin

type testServerPlugin struct {
	t               *testing.T
	name            string
	helpCalled      int
	configureCalled int
	startCalled     int
	stopCalled      int
	responseCalled  int
}

func (t *testServerPlugin) Name() string {
	if t.name != "" {
		return t.name
	}
	return "unit-test-1"
}

// PrintHelp prints the configuration help for the plugin.
func (t *testServerPlugin) PrintHelp(out io.Writer) {
	t.helpCalled++
	fmt.Fprintf(out, t.Name()+" help\n")
}

// Configure the plugin.
func (t *testServerPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	t.configureCalled++
	assert.NotNil(t.t, ctx)
	assert.NotNil(t.t, config)
	return nil
}

func (t *testServerPlugin) StartServer(ctx context.Context, handler plugins.Handler) error {
	t.startCalled++
	assert.NotNil(t.t, ctx)
	assert.NotNil(t.t, handler)
	return nil
}

// Stop the protocol plugin.
func (t *testServerPlugin) StopServer(ctx context.Context) error {
	t.stopCalled++
	assert.NotNil(t.t, ctx)
	return nil
}

func (t *testServerPlugin) Response(ctx context.Context, msg *dns.Msg) error {
	t.responseCalled++
	assert.NotNil(t.t, ctx)
	assert.NotNil(t.t, msg)
	return nil
}

// Client Plugin

type testClientPlugin struct {
	t               *testing.T
	name            string
	helpCalled      int
	configureCalled int
	startCalled     int
	stopCalled      int
	queryCalled     int
}

func (t *testClientPlugin) Name() string {
	if t.name != "" {
		return t.name
	}
	return "unit-test-1"
}

// PrintHelp prints the configuration help for the plugin.
func (t *testClientPlugin) PrintHelp(out io.Writer) {
	t.helpCalled++
	fmt.Fprintf(out, t.Name()+" help\n")
}

// Configure the plugin.
func (t *testClientPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	t.configureCalled++
	assert.NotNil(t.t, ctx)
	assert.NotNil(t.t, config)
	return nil
}

func (t *testClientPlugin) StartClient(ctx context.Context, handler plugins.Handler) error {
	t.startCalled++
	assert.NotNil(t.t, ctx)
	assert.NotNil(t.t, handler)
	return nil
}

// Stop the protocol plugin.
func (t *testClientPlugin) StopClient(ctx context.Context) error {
	t.stopCalled++
	assert.NotNil(t.t, ctx)
	return nil
}

func (t *testClientPlugin) Query(ctx context.Context, msg *dns.Msg) error {
	t.queryCalled++
	assert.NotNil(t.t, ctx)
	assert.NotNil(t.t, msg)
	return nil
}
