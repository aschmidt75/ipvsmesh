package daemon

import (
	"container/list"
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// ServiceWorker takes care about a single service of the
// current configuration model.
type ServiceWorker struct {
	StoppableByChan

	cfg     *model.IPVSMeshConfig
	service *model.Service
}

var (
	serviceWorkerList *list.List
)

// GetAllServiceWorkers returns a list of all services
// workers currently active
func GetAllServiceWorkers() *list.List {
	if serviceWorkerList == nil {
		serviceWorkerList = list.New()
	}
	return serviceWorkerList
}

// GetServiceWorkerByName retrieves a single ServiceWorker
// by the name of its service within the model.
func GetServiceWorkerByName(name string) *ServiceWorker {
	l := GetAllServiceWorkers()
	for e := l.Front(); e != nil; e = e.Next() {
		sw := e.Value.(*ServiceWorker)
		if sw.service.Name == name {
			return sw
		}
	}
	return nil
}

// NewServiceWorker creates a new ServiceWorker for a single service of a configuration model.
func NewServiceWorker(sc chan *sync.WaitGroup, cfg *model.IPVSMeshConfig, service *model.Service) *ServiceWorker {
	sw := &ServiceWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		cfg:     cfg,
		service: service,
	}
	GetAllServiceWorkers().PushBack(sw)
	return sw
}

func (s *ServiceWorker) queryAndProcessDownwardData() {
	p := s.service.Plugin
	data, err := p.GetDownwardData()
	if err != nil {
		log.WithField("err", err).Error("Unable to get downward data from plugin")
	}
	log.WithField("data", data).Info("Got data")
}

// Worker checks downward notifications
func (s *ServiceWorker) Worker() {
	log.WithField("Name", s.service.Name).Info("Starting service worker...")

	s.queryAndProcessDownwardData()

	updateCh := make(chan struct{})
	p := s.service.Plugin
	if p != nil {
		if p.HasDownwardInterface() {
			// set up notification
			go p.RunNotificationLoop(updateCh)
		}
	}

	for {
		select {
		case <-updateCh:
			s.queryAndProcessDownwardData()

		case wg := <-*s.StoppableByChan.StopChan:
			log.WithField("Name", s.service.Name).Info("Stopping service worker")
			wg.Done()
			updateCh <- struct{}{}
			return
		}
	}
}

// Update applies configuration updates to a ServiceWorker
func (s *ServiceWorker) Update(newService *model.Service) {
	log.WithField("Name", s.service.Name).Info("Updating service...")
	// TODO: apply new parts here..
	s.service = newService
	log.WithField("data", s.service).Info("Updated service.")
}
