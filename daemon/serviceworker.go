package daemon

import (
	"container/list"
	"sort"
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// ServiceWorker takes care about a single service of the
// current configuration model.
type ServiceWorker struct {
	StoppableByChan

	ipvsUpdateChan IPVSApplierChanType

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
func NewServiceWorker(sc chan *sync.WaitGroup, cfg *model.IPVSMeshConfig, service *model.Service, ipvsUpdateChan IPVSApplierChanType) *ServiceWorker {
	sw := &ServiceWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		cfg:            cfg,
		service:        service,
		ipvsUpdateChan: ipvsUpdateChan,
	}
	GetAllServiceWorkers().PushBack(sw)
	return sw
}

type byAddress []model.DownwardBackendServer

func (a byAddress) Len() int           { return len(a) }
func (a byAddress) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byAddress) Less(i, j int) bool { return a[i].Address < a[j].Address }

func (s *ServiceWorker) queryAndProcessDownwardData() {
	p := s.service.Plugin

	data, err := p.GetDownwardData()
	if err != nil {
		log.WithFields(log.Fields{
			"err":     err,
			"service": s.service.Name,
		}).Error("serviceworker: Unable to get downward data from plugin")
	}
	log.WithFields(log.Fields{
		"data":    data,
		"service": s.service.Name,
	}).Info("serviceworker: Received backend updates")

	// sort by address
	sort.Sort(byAddress(data))

	// forward them
	s.ipvsUpdateChan <- IPVSApplierUpdateStruct{
		serviceName: s.service.Name,
		service:     s.service,
		data:        data,
		cfg:         s.cfg,
	}
}

// Worker checks downward notifications
func (s *ServiceWorker) Worker() {
	log.WithField("Name", s.service.Name).Info("serviceworker: Starting service worker...")

	s.queryAndProcessDownwardData()

	updateCh := make(chan struct{})
	quitCh := make(chan struct{})
	p := s.service.Plugin
	if p != nil {
		if p.HasDownwardInterface() {
			// set up notification
			go p.RunNotificationLoop(updateCh, quitCh)
		}
	}

	for {
		select {
		case <-updateCh:
			s.queryAndProcessDownwardData()

		case wg := <-*s.StoppableByChan.StopChan:
			log.WithField("Name", s.service.Name).Info("serviceworker: Stopping service worker")
			wg.Done()
			quitCh <- struct{}{}
			return
		}
	}
}

// Update applies configuration updates to a ServiceWorker
func (s *ServiceWorker) Update(newService *model.Service) {
	log.WithField("Name", s.service.Name).Info("serviceworker: Updating service...")
	// TODO: apply new parts here..
	s.service = newService
	s.queryAndProcessDownwardData()
	log.WithField("data", s.service).Info("serviceworker: Updated service.")
}
