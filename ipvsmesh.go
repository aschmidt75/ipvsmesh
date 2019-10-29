package main

import (
	"os"

	"github.com/aschmidt75/ipvsmesh/cmd"
	"github.com/aschmidt75/ipvsmesh/config"
	"github.com/aschmidt75/ipvsmesh/logging"
	cli "github.com/jawher/mow.cli"
	log "github.com/sirupsen/logrus"
)

// Version is the cli app version
var Version string

func main() {

	c := config.Config()

	app := cli.App("ipvsmesh", "...")

	app.Version("version", Version)

	app.Spec = "[-d] [-v] [--trace] [--tls --tlscert=<certfile>] [--tlskey=<keyfile>]"

	trace := app.BoolOpt("trace", c.Trace, "Show trace messages")
	debug := app.BoolOpt("d debug", c.Debug, "Show debug messages")
	verbose := app.BoolOpt("v verbose", c.Verbose, "Show more information")
	tls := app.BoolOpt("tls", false, "Use TLS for daemon communication. Needs --tlscert and --tlskey")
	tlscert := app.StringOpt("tlscert", "", "TLS certificate file in PEM format. Valid only with --tls and --tlskey")
	tlskey := app.StringOpt("tlskey", "", "TLS key file in PEM format. Valid only with --tls and --tlskcert")

	app.Command("daemon", "manages the background daemon.", cmd.Daemon)

	app.Before = func() {
		if trace != nil {
			c.Trace = *trace
		}
		if debug != nil {
			c.Debug = *debug
		}
		if verbose != nil {
			c.Verbose = *verbose
		}
		logging.InitLogging(c.Trace, c.Debug, c.Verbose)

		c.TLS = *tls
		if c.TLS {
			if *tlscert == "" {
				log.Fatal("Missing --tlscert. Please supply when using --tls.")
			}

			c.TLSCertFile = *tlscert
			c.TLSKeyFile = *tlskey
		}
	}
	app.Run(os.Args)
}
