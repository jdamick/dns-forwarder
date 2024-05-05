package plugins

import (
	"context"
	"fmt"
	"io"

	"github.com/miekg/dns"
)

type QueryLoggerPlugin struct {
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&QueryLoggerPlugin{})
}

func (q *QueryLoggerPlugin) Name() string {
	return "querylogger"
}

// PrintHelp prints the configuration help for the plugin.
func (q *QueryLoggerPlugin) PrintHelp(out io.Writer) {

}

// Configure the plugin.
func (q *QueryLoggerPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (q *QueryLoggerPlugin) Query(ctx context.Context, msg *dns.Msg) error {
	fmt.Printf("Query: \n%v\n", msg.String())
	return nil
}
