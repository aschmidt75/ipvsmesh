package logging

import (
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	once sync.Once
)

// InitLogging creates a new logger based on verbosity and settings
func InitLogging(trace, debug, verbose bool) {
	once.Do(func() {

		log.SetLevel(log.ErrorLevel)
		log.SetOutput(os.Stdout)

		if verbose {
			log.SetLevel(log.InfoLevel)
		}
		if debug {
			log.SetLevel(log.DebugLevel)
		}
		if trace {
			log.SetLevel(log.TraceLevel)
		}

		// file

		log.SetFormatter(&log.TextFormatter{})
	})
}
