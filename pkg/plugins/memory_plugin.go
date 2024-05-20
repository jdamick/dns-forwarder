package plugins

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/inhies/go-bytesize"
	utils "github.com/jdamick/dns-forwarder/pkg/utils"
	log "github.com/rs/zerolog/log"
)

type MemoryPlugin struct {
	config MemoryPluginConfig
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&MemoryPlugin{})
}

func (m *MemoryPlugin) Name() string {
	return "memory"
}

// PrintHelp prints the configuration help for the plugin.
func (c *MemoryPlugin) PrintHelp(out io.Writer) {
	PrintPluginHelp(c.Name(), &c.config, out)
}

type MemoryPluginConfig struct {
	Cap      string `toml:"cap" comment:"Cap Memory Use, either size (10MB) or % of available" default:"0b"`
	capBytes bytesize.ByteSize
}

// Configure the plugin.
func (m *MemoryPlugin) Configure(ctx context.Context, config map[string]interface{}) (err error) {
	log.Debug().Any("config", config).Msg("MemoryPlugin.Configure")
	if err := UnmarshalConfiguration(config, &m.config); err != nil {
		return err
	}
	if strings.Contains(m.config.Cap, "%") {
		result := strings.Split(m.config.Cap, "%")
		if len(result) != 2 {
			return fmt.Errorf("invalid memory cap %s", m.config.Cap)
		}
		val, err := strconv.ParseFloat(result[0], 64)
		if err != nil {
			return err
		}
		mem := utils.CurrentMemoryInUse() + utils.FreeMemory()
		m.config.capBytes = bytesize.ByteSize(uint64(val * 0.01 * float64(mem)))
	} else {
		m.config.capBytes, err = bytesize.Parse(m.config.Cap)
	}
	// if capped, tune the gc
	if m.config.capBytes > 0 {
		log.Info().Stringer("total system memory", bytesize.ByteSize(utils.TotalMemory())).Send()
		log.Info().Stringer("available system memory", bytesize.ByteSize(utils.FreeMemory())).Send()
		log.Info().Stringer("Cap", m.config.capBytes).Msg("setting memory limit")
		debug.SetMemoryLimit(int64(m.config.capBytes))
		debug.SetGCPercent(-1)
	}

	return nil
}
