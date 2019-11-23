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

	updateChan     ConfigUpdateChanType
	ipvsUpdateChan IPVSApplierChanType
	wg             sync.WaitGroup
	scWorkers      chan *sync.WaitGroup
}

// NewConfigApplierWorker creates a Configuration applier worker based on
// an update channel.
func NewConfigApplierWorker(updateChan ConfigUpdateChanType, ipvsUpdateChan IPVSApplierChanType) *ConfigApplierWorker {
	sc := make(chan *sync.WaitGroup, 1)
	scWorkers := make(chan *sync.WaitGroup, 1)

	return &ConfigApplierWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		updateChan:     updateChan,
		scWorkers:      scWorkers,
		ipvsUpdateChan: ipvsUpdateChan,
	}
}

func (s *ConfigApplierWorker) Worker() {
	log.Info("Starting Configuration applier...")
	for {
		select {
		case cfg := <-s.updateChan:
			log.WithField("cfg", cfg).Debug("Received new config")
			err := s.applyServices(cfg)
			if err != nil {
				log.WithField("err", err).Error("Unable to apply configuration (services)")
			}

			err = s.applyPublishers(cfg)
			if err != nil {
				log.WithField("err", err).Error("Unable to apply configuration (publishers)")
			}

			log.WithField("numServicesActive", GetAllServiceWorkers().Len()).Info("Applied new configuration")

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("Stopping Configuration Applier")

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
	log.Debug("Applying services...")

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
		sw = NewServiceWorker(s.scWorkers, &cfg, service, s.ipvsUpdateChan)
		s.wg.Add(1)
		go sw.Worker()
	} else {
		sw.Update(service)
	}
	log.WithField("sw", sw).Debug("Activated/Updated service worker")

	return nil
}

func (s *ConfigApplierWorker) applyPublishers(cfg model.IPVSMeshConfig) error {
	log.Debug("Applying publishers...")

	m := make(map[string]*model.Publisher)
	for _, publisher := range cfg.Publishers {
		m[publisher.Name] = publisher
	}

	// walk active publisher worker, check if they're still part of the model.
	l := GetAllPublisherhWorkers()
	for e := l.Front(); e != nil; e = e.Next() {
		pw := e.Value.(*PublisherhWorker)
		_, ex := m[pw.publisher.Name]
		if !ex {
			log.WithField("name", pw.publisher.Name).Info("Taking down because not part of model any more")
			*pw.StopChan <- &s.wg
			l.Remove(e)
		}
	}

	// walk new config, add/update workers
	for _, publisher := range cfg.Publishers {
		if err := s.applyPublisher(cfg, publisher); err != nil {
			return err
		}
	}

	return nil
}

func (s *ConfigApplierWorker) applyPublisher(cfg model.IPVSMeshConfig, publisher *model.Publisher) error {
	log.WithField("name", publisher.Name).Debug("Applying publisher...")

	pw := GetPublisherWorkerByName(publisher.Name)
	if pw == nil {
		// create
		pw = NewPublisherWorker(s.scWorkers, &cfg, publisher)
		s.wg.Add(1)
		go pw.Worker()
	} else {
		pw.Update(publisher)
	}
	log.WithField("pw", pw).Debug("Activated/Updated publisher worker")

	return nil
}
