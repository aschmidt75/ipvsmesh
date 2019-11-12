package config

import (
	"github.com/caarlos0/env/v6"
)

// Configuration holds all global config entries
type Configuration struct {
	Trace                 bool   `env:"IPVSMESH_LOG_TRACE" envDefault:"false"`
	Debug                 bool   `env:"IPVSMESH_LOG_DEBUG" envDefault:"false"`
	Verbose               bool   `env:"IPVSMESH_LOG_VERBOSE" envDefault:"false"`
	DaemonizeFlag         bool   `env:"IPVSMESH_DAEMONIZE" envDefault:"false"`
	DaemonSocketPath      string `env:"IPVSMESH_SOCKET" envDefault:"/tmp/ipvsmesh.sock"`
	DaemonConnTimeoutSecs int    `env:"IPVSMESH_DAEMON_TIMEOUT_SEC" envDefault:"5"`

	DefaultConfigFile string `env:"IPVSMESH_CONFIGFILE" envDefault:"/etc/ipvsmesh.yaml"`

	TLS         bool   `env:"IPVSMESH_TLS" envDefault:"false"`
	TLSCertFile string `env:"IPVSMESH_TLSCERTFILE" envDefault:""`
	TLSKeyFile  string `env:"IPVSMESH_TLSKEYFILE" envDefault:""`
}

var (
	configuration *Configuration
)

// Config retrieves the current configuration
func Config() *Configuration {
	if configuration == nil {
		configuration = &Configuration{}

		// parse env
		if err := env.Parse(configuration); err != nil {
			panic(err)
		}
	}
	return configuration
}
