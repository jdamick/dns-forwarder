package dnsforwarder

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	plugins "github.com/jdamick/dns-forwarder/pkg/plugins"
	"github.com/kardianos/service"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	log "github.com/rs/zerolog/log"
	pkgerrors "github.com/rs/zerolog/pkgerrors"
)

const name = "dns-forwarder"

var (
	GitCommit string
)

func init() {
}

func setupLogging() {
	wr := diode.NewWriter(os.Stdout, 1000, 0, func(missed int) {
		fmt.Printf("Logger Dropped %d messages", missed)
	})
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: wr})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

var (
	createService = func(svc service.Interface, conf *service.Config) (service.Service, error) {
		return service.New(svc, conf)
	}
)

func ForwarderMain() {
	setupLogging()

	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.SetOutput(os.Stdout)
	svcFlag := fs.String("service", "", "Control the system service ("+strings.Join(service.ControlAction[:], ", ")+")")
	logLevel := fs.String("loglevel", "info", "Log level (debug, info, warn, error, fatal, panic)")
	configFile := fs.String("config", name+".toml", "Configuration file (dns-forwarder.toml)")
	pluginHelp := fs.String("pluginConfig", "", "Print configuration help for a plugin")
	version := fs.Bool("version", false, "Print version")
	listPlugins := fs.Bool("listPlugins", false, "List available plugins")
	fs.Parse(os.Args[1:])

	lvl, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	zerolog.SetGlobalLevel(lvl)
	if lvl == zerolog.DebugLevel {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	}

	if pluginHelp != nil && *pluginHelp != "" {
		NewForwarder().PrintHelp(*pluginHelp, os.Stdout)
		return
	}
	if version != nil && *version {
		fmt.Printf("\n%v git version: %v\n\n", filepath.Base(os.Args[0]), GitCommit)
		return
	}
	if listPlugins != nil && *listPlugins {
		plugins.PrintPlugins[plugins.Plugin](os.Stdout)
		temp := NewForwarder()
		fmt.Printf("\nPlugin Configurations:\n")
		plugins.RunForAllPlugins(func(p plugins.Plugin) (err error) {
			temp.PrintHelp(p.Name(), os.Stdout)
			return
		})
		return
	}

	svcConfig := &service.Config{
		Name:        name,
		DisplayName: "DNS Forwarder",
		Description: "DNS Forwarder",
	}

	dnsSrvr := &DNSForwarderService{configFile: *configFile}
	s, err := createService(dnsSrvr, svcConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("service creation failed")
	}
	if len(*svcFlag) != 0 {
		err = service.Control(s, *svcFlag)
		if err != nil {
			log.Fatal().Err(err).Msg("service control failed")
		}
		return
	}

	err = s.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("service run failed")
	}
}

type DNSForwarderService struct {
	configFile string
	forwarder  *Forwarder
}

func (p *DNSForwarderService) Start(s service.Service) error {
	p.forwarder = NewForwarder()
	c, err := os.Open(p.configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open file")
	}
	defer c.Close()
	err = p.forwarder.ConfigureFrom(c)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to configure")
	}

	err = p.forwarder.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start")
	}
	return nil
}

func (p *DNSForwarderService) Stop(s service.Service) error {
	log.Debug().Msg("Stopping")
	p.forwarder.Stop()
	return nil
}
