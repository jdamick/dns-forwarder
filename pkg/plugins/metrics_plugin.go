package plugins

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/VictoriaMetrics/metrics"
	log "github.com/rs/zerolog/log"
)

const (
	defaultMetricsPort = 8080
)

type MetricsPlugin struct {
	config MetricsPluginConfig
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&MetricsPlugin{})
}

func (q *MetricsPlugin) Name() string {
	return "metrics"
}

// PrintHelp prints the configuration help for the plugin.
func (c *MetricsPlugin) PrintHelp(out io.Writer) {
	PrintPluginHelp(c.Name(), &c.config, out)
}

type MetricsPluginConfig struct {
	Port int `json:"port" comment:"Metrics HTTP Port"`
}

// Configure the plugin.
func (c *MetricsPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	log.Debug().Any("config", config).Msg("MetricsPlugin.Configure")
	// set defaults
	c.config.Port = defaultMetricsPort
	if err := UnmarshalConfiguration(config, &c.config); err != nil {
		return err
	}
	go c.StartHttpEndpoint()
	return nil
}

func (c *MetricsPlugin) StartHttpEndpoint() {

	mux := http.NewServeMux()
	// Expose the registered metrics at `/metrics` path.
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	listenAddr := fmt.Sprintf("%v:%v", "", c.config.Port)
	// creating some helper variables to avoid data races on m.srv and m.ln
	server := &http.Server{Addr: listenAddr, Handler: mux}
	//m.srv = server

	go func() {
		server.ListenAndServe()
	}()

}
