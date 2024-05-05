package main

import (
	"os"

	dnsforwarder "github.com/jdamick/dns-forwarder/pkg"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
	pkgerrors "github.com/rs/zerolog/pkgerrors"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
}

func main() {

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if true {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	}

	f := dnsforwarder.NewForwarder()
	c, err := os.Open("test.json")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open file")
	}
	defer c.Close()
	err = f.ConfigureFrom(c)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to configure")
	}

	err = f.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start")
	}
	// wait until shutdown..
	done := make(chan bool)
	<-done
}
