package daemon

import (
	"sync"
	"time"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// ConfigUpdateChanType is a channel that transmits the model
type ConfigUpdateChanType chan model.IPVSMeshConfig

// ConfigApplierWorker is a worker listening on the update channel
// and applying new configurations.
type ConfigApplierWorker struct {
	StoppableByChan

	updateChan ConfigUpdateChanType
	wg         sync.WaitGroup
	scWorkers  chan *sync.WaitGroup
}

// NewConfigApplierWorker creates a Configuration applier worker based on
// an update channel.
func NewConfigApplierWorker(updateChan ConfigUpdateChanType) *ConfigApplierWorker {
	sc := make(chan *sync.WaitGroup, 1)
	scWorkers := make(chan *sync.WaitGroup, 1)

	return &ConfigApplierWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		updateChan: updateChan,
		scWorkers:  scWorkers,
	}
}

func (s *ConfigApplierWorker) Worker() {
	log.Info("Starting Configuration applier...")
	for {
		select {
		case cfg := <-s.updateChan:
			log.WithField("cfg", cfg).Info("Received new config")
			err := s.applyServices(cfg)
			if err != nil {
				log.WithField("err", err).Error("Unable to apply configuration")
			}

			log.WithField("numServicesActive", GetAllServiceWorkers().Len()).Debug("stat")

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("Configuratiom Applier stopping")

			<-time.After(1 * time.Second)
			wg.Done()
			return
		}
	}
}

func (s *ConfigApplierWorker) applyServices(cfg model.IPVSMeshConfig) error {
	log.Debug("Applying services...")

	m := make(map[string]*model.Service)
	for _, service := range cfg.Services {
		m[service.Name] = service
	}

	// walk active service worker, check if they're still part of the model.
	l := GetAllServiceWorkers()
	for e := l.Front(); e != nil; e = e.Next() {
		sw := e.Value.(*ServiceWorker)
		_, ex := m[sw.service.Name]
		if !ex {
			log.WithField("name", sw.service.Name).Info("Taking down because not part of model any more")
			*sw.StopChan <- &s.wg
			l.Remove(e)
		}
	}

	// walk new config, add/update workers
	for _, service := range cfg.Services {
		if err := s.applyService(cfg, service); err != nil {
			return err
		}
	}

	return nil
}

func (s *ConfigApplierWorker) applyService(cfg model.IPVSMeshConfig, service *model.Service) error {
	log.WithField("name", service.Name).Debug("Applying service...")

	sw := GetServiceWorkerByName(service.Name)
	if sw == nil {
		// create
		sw = NewServiceWorker(s.scWorkers, &cfg, service)
		s.wg.Add(1)
		go sw.Worker()
	} else {
		sw.Update(service)
	}
	log.WithField("sw", sw).Debug("Activated/Updated service worker")

	return nil
}
