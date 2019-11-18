package daemon

import (
	"os"
	"sync"
	"time"

	"github.com/aschmidt75/ipvsmesh/config"
	"github.com/aschmidt75/ipvsmesh/plugins"
	"github.com/radovskyb/watcher"
	log "github.com/sirupsen/logrus"
)

type ConfigWatcherWorker struct {
	StoppableByChan

	configFileName string
	lastModTime    time.Time
	updateChan     ConfigUpdateChanType
}

// NewConfigWatcherWorker creates a new watcher on given config file. It reads
// changes and sends updates to updateChan
func NewConfigWatcherWorker(configFileName string, updateChan ConfigUpdateChanType) *ConfigWatcherWorker {
	sc := make(chan *sync.WaitGroup, 1)
	return &ConfigWatcherWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		configFileName: configFileName,
		updateChan:     updateChan,
	}
}

func (s *ConfigWatcherWorker) Worker() {
	log.Info("Starting Configuration watcher...")

	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write, watcher.Create, watcher.Remove)

	if err := w.Add(s.configFileName); err != nil {
		log.WithField("err", err).Error("Unable to set up watcher")
	}

	go func() {

		// Trigger artificial event to have config read the first time
		// we get here.
		w.Wait()
		log.Debug("Initial config file read trigger")
		w.TriggerEvent(watcher.Create, nil)
	}()

	go func() {
		if err := w.Start(time.Millisecond * 100); err != nil {
			log.WithField("err", err).Error("Unable to start watcher")
		}

	}()

	log.Debug("Processing file watcher updates.")
	for {
		select {
		case event := <-w.Event:
			log.WithField("e", event).Debug("config file(s) changed")
			info, err := os.Stat(s.configFileName)
			if err == nil {
				mt := info.ModTime()
				if mt.After(s.lastModTime) {
					s.readConfig()
					s.lastModTime = mt
				}
			}
		case err := <-w.Error:
			log.Errorln(err)
		case <-w.Closed:
			log.Info("Stopping Configuratiom Watcher")
			return
		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("Stopping Configuratiom Watcher")
			wg.Done()
			return
		}
	}

}

func (s *ConfigWatcherWorker) readConfig() {
	log.Debug("Reading input file")

	// read my config file
	cfg, err := config.ReadModelFromInput(s.configFileName)
	if err != nil {
		log.Error(err)
		return
	}
	log.WithField("cfg", *cfg).Debug("Read config")

	// walk over services, parse spec fields according to plugins

	for _, service := range cfg.Services {
		spec, err := plugins.ReadPluginSpecByTypeString(service)
		if err != nil {
			log.WithField("err", err).Errorf("Unable to parse spec for service %s", service.Name)
			continue
		}
		log.WithFields(log.Fields{
			"spec": spec,
			"name": spec.Name(),
		}).Debug("spec")
		service.Plugin = spec
	}

	// send new config to update channel
	s.updateChan <- *cfg
}
