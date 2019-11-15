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
			//		case <-time.After(1 * time.Second):
			p := s.service.Plugin
			data, err := p.GetDownwardData()
			if err != nil {
				log.WithField("err", err).Error("Unable to get downward data from plugin")
			}
			log.WithField("data", data).Info("Got data")

		case wg := <-*s.StoppableByChan.StopChan:
			log.WithField("Name", s.service.Name).Info("Stopping service worker")
			wg.Done()
			updateCh <- struct{}{}
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
