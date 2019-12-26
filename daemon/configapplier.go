package daemon

import (
	"sync"
	"time"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// ConfigUpdateChanType is a channel that transmits updated configuration
type ConfigUpdateChanType chan model.IPVSMeshConfig

// ConfigApplierWorker is a worker listening on the update channel
// and applying new configurations.
type ConfigApplierWorker struct {
	StoppableByChan

	updateChan                ConfigUpdateChanType
	ipvsUpdateChan            IPVSApplierChanType
	publisherConfigUpdateChan PublisherConfigUpdateChanType
	wg                        sync.WaitGroup
	scWorkers                 chan *sync.WaitGroup
}

// NewConfigApplierWorker creates a Configuration applier worker based on
// an update channel.
func NewConfigApplierWorker(updateChan ConfigUpdateChanType, ipvsUpdateChan IPVSApplierChanType, publisherConfigUpdateChan PublisherConfigUpdateChanType) *ConfigApplierWorker {
	sc := make(chan *sync.WaitGroup, 1)
	scWorkers := make(chan *sync.WaitGroup, 1)

	return &ConfigApplierWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		updateChan:                updateChan,
		scWorkers:                 scWorkers,
		ipvsUpdateChan:            ipvsUpdateChan,
		publisherConfigUpdateChan: publisherConfigUpdateChan,
	}
}

func (s *ConfigApplierWorker) Worker() {
	log.Info("configapplier: Starting Configuration applier...")
	for {
		select {
		case cfg := <-s.updateChan:
			log.WithField("cfg", cfg).Debug("configapplier: Received new config")

			// apply config to publisher worker
			s.publisherConfigUpdateChan <- cfg

			err := s.applyServices(cfg)
			if err != nil {
				log.WithField("err", err).Error("configapplier: Unable to apply configuration (services)")
			}

			// done.
			log.WithField("numServicesActive", GetAllServiceWorkers().Len()).Info("configapplier: Applied new configuration")

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("configapplier: Stopping")

			// stop all active service workers
			l := GetAllServiceWorkers()
			for e := l.Front(); e != nil; e = e.Next() {
				sw := e.Value.(*ServiceWorker)
				*sw.StopChan <- &s.wg
				l.Remove(e)
			}

			<-time.After(1 * time.Second)
			wg.Done()
			return
		}
	}
}

func (s *ConfigApplierWorker) applyServices(cfg model.IPVSMeshConfig) error {
	log.Debug("configapplier: Applying services...")

	// force ipvsapplier to clear caches
	s.ipvsUpdateChan <- IPVSApplierUpdateStruct{
		serviceName: "",
	}

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
			log.WithField("name", sw.service.Name).Debug("configapplier: Taking down because not part of model any more")
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
	log.WithField("name", service.Name).Debug("configapplier: Applying service...")

	sw := GetServiceWorkerByName(service.Name)
	if sw == nil {
		// create
		sw = NewServiceWorker(s.scWorkers, &cfg, service, s.ipvsUpdateChan)
		s.wg.Add(1)
		go sw.Worker()
	} else {
		sw.Update(service)
	}
	log.WithField("sw", sw).Debug("configapplier: Activated/Updated service worker")

	return nil
}
