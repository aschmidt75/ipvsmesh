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

// ConfigWatcherWorker is a continuously running loop
// watching changes on a given config file. If the file
// changes, it is read, parsed and passed on to an
// update channel.
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
	log.Info("configwatcher: Starting Configuration watcher...")

	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write, watcher.Create, watcher.Remove)

	if err := w.Add(s.configFileName); err != nil {
		log.WithField("err", err).Error("configwatcher: Unable to set up watcher")
	}

	go func() {

		// Trigger artificial event to have config read the first time
		// we get here.
		w.Wait()
		log.Debug("configwatcher: Initial config file read trigger")
		w.TriggerEvent(watcher.Create, nil)
	}()

	go func() {
		if err := w.Start(time.Millisecond * 100); err != nil {
			log.WithField("err", err).Error("configwatcher: Unable to start watcher")
		}

	}()

	log.Debug("configwatcher: Processing file watcher updates.")
	for {
		select {
		case event := <-w.Event:
			log.WithField("e", event).Debug("configwatcher: config file(s) changed")
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
			log.Info("configwatcher: Stopping Configuratiom Watcher")
			return
		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("configwatcher: Stopping Configuratiom Watcher")
			wg.Done()
			return
		}
	}

}

func (s *ConfigWatcherWorker) readConfig() {
	log.Debug("configwatcher: Reading input file")

	// read my config file
	cfg, err := config.ReadModelFromInput(s.configFileName)
	if err != nil {
		log.Error(err)
		return
	}
	log.WithField("cfg", *cfg).Debug("configwatcher: Read config")

	// walk over services, parse spec fields according to plugins

	ok := true
	for _, service := range cfg.Services {
		spec, err := plugins.ReadPluginSpecByTypeString(service)
		if err != nil {
			log.WithField("err", err).Errorf("configwatcher: Unable to parse spec for service %s", service.Name)
			ok = false
			continue
		}
		log.WithFields(log.Fields{
			"spec": spec,
			"name": spec.Name(),
		}).Trace("configwatcher: service spec")

		spec.Initialize(&cfg.Globals)
		service.Plugin = spec

	}
	for _, publisher := range cfg.Publishers {
		var err error
		spec, err := plugins.ReadPublisherPluginSpecByTypeString(publisher)
		if err != nil {
			log.WithField("err", err).Error("unable to parse spec for publisher %s", publisher.Name)
			ok = false
		}
		log.WithFields(log.Fields{
			"spec": spec,
			"name": spec.Name(),
		}).Trace("configwatcher: publisher spec")

		spec.Initialize(&cfg.Globals)
		publisher.Plugin = spec
	}

	if !ok {
		log.Warn("configwatcher: There are configuration errors, will not apply this.")
		return
	}

	// inject refs to globals to all services and publishers
	for _, service := range cfg.Services {
		service.Globals = &cfg.Globals
	}
	for _, publisher := range cfg.Publishers {
		publisher.Globals = &cfg.Globals
	}

	// send new config to update channel
	s.updateChan <- *cfg
}
