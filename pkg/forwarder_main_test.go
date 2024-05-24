package dnsforwarder

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/jdamick/dns-forwarder/pkg/plugins"
	"github.com/kardianos/service"
	"github.com/stretchr/testify/assert"
)

func TestForwarderMain(t *testing.T) {

	var fs *fakeService
	createService = func(i service.Interface, c *service.Config) (service.Service, error) {
		s, _ := service.New(i, c)
		fs = &fakeService{real: s, intf: i}
		return fs, nil
	}

	assert := assert.New(t)

	f := createTestConfigFile()
	assert.NotNil(f)
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	var pluginWG sync.WaitGroup
	pluginWG.Add(1)
	plugins.RegisterPlugin(&TestPlugin{wg: &pluginWG})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		args := os.Args
		defer func() { os.Args = args }()
		os.Args = []string{"dns-forwarder", "-config", f.Name()}
		ForwarderMain()
	}()

	pluginWG.Wait()

	// stop the forwarder
	fs.Stop()

	wg.Wait()
	assert.Equal(1, fs.runCalled)
	assert.Equal(1, fs.stopCalled)
}

func TestForwarderHelp(t *testing.T) {
	tp := &TestPlugin{name: "unit-test-help"}
	plugins.RegisterPlugin(tp)
	assert := assert.New(t)

	args := osArgHolder()
	defer args()
	os.Args = []string{"dns-forwarder", "-listPlugins"}
	ForwarderMain()
	assert.Equal(1, tp.helpCalled)
}

// Test Mocks & Helpers
///////////////////////

func osArgHolder() func() {
	args := os.Args
	return func() { os.Args = args }
}

type TestPlugin struct {
	wg         *sync.WaitGroup
	name       string
	helpCalled int
}

func (t *TestPlugin) Name() string {
	if t.name != "" {
		return t.name
	}
	return "unit-test-1"
}

// PrintHelp prints the configuration help for the plugin.
func (t *TestPlugin) PrintHelp(out io.Writer) {
	t.helpCalled++
	fmt.Fprintf(out, t.Name()+" help\n")
}

// Configure the plugin.
func (t *TestPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	if t.wg != nil {
		t.wg.Done()
	}
	return nil
}

func createTestConfigFile() *os.File {
	file, err := os.CreateTemp("", "dns-forwarder.toml")
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(file, `
[unit-test-1]
`)
	file.Sync()
	return file
}

type fakeService struct {
	real            service.Service
	intf            service.Interface
	runDone         chan struct{}
	runCalled       int
	startCalled     int
	stopCalled      int
	restartCalled   int
	installCalled   int
	uninstallCalled int
}

func (f *fakeService) Run() error {
	if f.runDone == nil {
		f.runDone = make(chan struct{})
	}
	f.runCalled++
	f.intf.Start(nil)
	<-f.runDone
	return nil
}

func (f *fakeService) Start() error {
	f.startCalled++
	return nil
}

// Stop signals to the OS service manager the given service should stop.
func (f *fakeService) Stop() error {
	if f.runDone == nil {
		f.runDone = make(chan struct{})
	}
	f.stopCalled++
	f.intf.Stop(nil)
	f.runDone <- struct{}{}
	return nil
}

// Restart signals to the OS service manager the given service should stop then start.
func (f *fakeService) Restart() error {
	f.restartCalled++
	return nil
}

func (f *fakeService) Install() error {
	f.installCalled++
	return nil
}

func (f *fakeService) Uninstall() error {
	f.uninstallCalled++
	return nil
}

func (f *fakeService) Logger(errs chan<- error) (service.Logger, error) {
	return f.real.SystemLogger(errs)
}

func (f *fakeService) SystemLogger(errs chan<- error) (service.Logger, error) {
	return f.real.SystemLogger(errs)
}

func (f *fakeService) String() string {
	return f.real.String()
}

func (f *fakeService) Platform() string {
	return f.real.Platform()
}

func (f *fakeService) Status() (service.Status, error) {
	return f.real.Status()
}
