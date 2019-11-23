package daemon

import (
	"container/list"
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// PublisherhWorker takes care about a single service of the
// current configuration model.
type PublisherhWorker struct {
	StoppableByChan

	// TODO updateCh

	cfg       *model.IPVSMeshConfig
	publisher *model.Publisher
}

var (
	// PublishWorkerList contains all registered publishers
	PublishWorkerList *list.List
)

// GetAllPublisherhWorkers returns a list of all services
// workers currently active
func GetAllPublisherhWorkers() *list.List {
	if PublishWorkerList == nil {
		PublishWorkerList = list.New()
	}
	return PublishWorkerList
}

// GetPublisherWorkerByName retrieves a single PublishWorker
// by the name of its publisher within the model.
func GetPublisherWorkerByName(name string) *PublisherhWorker {
	l := GetAllPublisherhWorkers()
	for e := l.Front(); e != nil; e = e.Next() {
		sw := e.Value.(*PublisherhWorker)
		if sw.publisher.Name == name {
			return sw
		}
	}
	return nil
}

// NewPublisherWorker creates a new PublisherWorker for a single publisher of a configuration model.
func NewPublisherWorker(sc chan *sync.WaitGroup, cfg *model.IPVSMeshConfig, publisher *model.Publisher) *PublisherhWorker {
	sw := &PublisherhWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		cfg:       cfg,
		publisher: publisher,
	}
	GetAllPublisherhWorkers().PushBack(sw)
	return sw
}

// Worker checks downward notifications
func (s *PublisherhWorker) Worker() {
	log.WithField("Name", s.publisher.Name).Info("Starting publish worker...")

	for {
		select {
		//		case <-updateCh:

		case wg := <-*s.StoppableByChan.StopChan:
			log.WithField("Name", s.publisher.Name).Info("Stopping service worker")
			wg.Done()
			return
		}
	}
}

// Update applies configuration updates to a PublishWorker
func (s *PublisherhWorker) Update(newPublisher *model.Publisher) {
	log.WithField("Name", s.publisher.Name).Info("Updating publisher...")
	s.publisher = newPublisher
	// TODO: apply new parts here..

	log.WithField("data", s.publisher).Info("Updated publisher.")
}
