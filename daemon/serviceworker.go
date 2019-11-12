package daemon

import (
	"container/list"
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

type ServiceWorker struct {
	StoppableByChan

	cfg     *model.IPVSMeshConfig
	service *model.Service
}

var (
	serviceWorkerList *list.List
)

func GetAllServiceWorkers() *list.List {
	if serviceWorkerList == nil {
		serviceWorkerList = list.New()
	}
	return serviceWorkerList
}

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

func (s *ServiceWorker) Worker() {
	log.WithField("Name", s.service.Name).Info("Starting service worker...")
	for {
		select {

		case wg := <-*s.StoppableByChan.StopChan:
			log.WithField("Name", s.service.Name).Info("Stopping service worker")
			wg.Done()
			return
		}
	}
}

func (s *ServiceWorker) Update(newService *model.Service) {
	log.WithField("Name", s.service.Name).Info("Updating service...")
	// TODO: apply new parts here..
	s.service = newService
	log.WithField("data", s.service).Info("Updated service.")
}
