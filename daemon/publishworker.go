package daemon

import (
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// PublisherUpdate is a message from the ipvs applier indicating
// that an endpoint has changed
type PublisherUpdate struct {
	data map[string]interface{}
}

// PublisherUpdateChanType is for backend updates
type PublisherUpdateChanType chan PublisherUpdate

// PublisherConfigUpdateChanType is not configuration updates
type PublisherConfigUpdateChanType chan model.IPVSMeshConfig

// PublisherhWorker takes care about a single service of the
// current configuration model.
type PublisherhWorker struct {
	StoppableByChan

	updateCh       PublisherUpdateChanType
	configUpdateCh PublisherConfigUpdateChanType

	cfg model.IPVSMeshConfig

	// maps publisher names to their specs
	publisherSpecs map[string]*model.Publisher
}

// NewPublisherWorker creates a worker for publish notifications
func NewPublisherWorker(updateChan PublisherUpdateChanType, configUpdateCh PublisherConfigUpdateChanType) *PublisherhWorker {
	sc := make(chan *sync.WaitGroup, 1)
	return &PublisherhWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		updateCh:       updateChan,
		configUpdateCh: configUpdateCh,

		publisherSpecs: make(map[string]*model.Publisher, 5),
	}
}

func (s *PublisherhWorker) getLabelsByServiceName(name string) (map[string]string, bool) {
	for _, service := range s.cfg.Services {
		if service.Name == name {
			return service.Labels, true
		}
	}
	return make(map[string]string, 0), false
}

func (s *PublisherhWorker) findPublishersByLabels(matchLabels map[string]string) []*model.Publisher {
	res := make([]*model.Publisher, 0, 5)
	log.WithField("matchLabels", matchLabels).Trace("Looking for publishers with these labels")

	for publisherName, publisher := range s.publisherSpecs {
		/*		log.WithFields(log.Fields{
					"name":   publisherName,
					"labels": publisher.MatchLabels,
				}).Trace("Comparing publisher")
		*/
		found := true
		for k, v := range matchLabels {
			vv, ex := publisher.MatchLabels[k]
			if !ex || v != vv {
				found = false
			}
		}

		if found {
			log.WithFields(log.Fields{
				"name": publisherName,
				"l":    matchLabels,
			}).Debug("Found publisher by labels")
			res = append(res, publisher)
		}
	}

	return res
}

// walks over a PublisherUpdate, looks up affected
// services/publishers and triggers them
func (s *PublisherhWorker) walkUpdate(upd PublisherUpdate) error {
	servicesRaw, ex := upd.data["services"]
	if !ex {
		// no services there. Notify all publishers to take down existings endpoints
		log.Debug("PublisherWorker: No services")
		return nil
	}

	services := servicesRaw.([]interface{})

	for _, serviceRaw := range services {
		service := serviceRaw.(map[string]interface{})
		serviceName, ex := service["ipvsmesh.service.name"].(string)
		if !ex {
			log.WithField("serviceName", serviceName).Error("Internal error, unable to track ipvsctl service.")
			continue
		}

		// look up labels of service
		labels, found := s.getLabelsByServiceName(serviceName)
		if !found {
			log.WithField("serviceName", serviceName).Error("Internal error, unable to find ipvsctl service (2).")
			continue
		}
		log.WithField("labels", labels).Trace("Labels for service name")

		// look up publishers with these labels
		publishers := s.findPublishersByLabels(labels)
		log.WithFields(log.Fields{
			"num":         len(publishers),
			"serviceName": serviceName,
		}).Debug("Found publishers for matchLabels")
	}

	return nil
}

// Worker checks downward notifications
func (s *PublisherhWorker) Worker() {
	log.Info("Starting publish worker...")

	for {
		select {
		case upd := <-s.updateCh:
			log.WithField("upd", upd).Debug("PublisherWorker: got backend update")

			err := s.walkUpdate(upd)
			if err != nil {
				log.WithField("err", err).Error("Unable to publish configuration update")
			}

		case cfg := <-s.configUpdateCh:
			log.WithField("numPublishers", len(cfg.Publishers)).Debug("PublisherWorker: got config update")

			s.cfg = cfg

			for _, publisher := range cfg.Publishers {
				_, ex := s.publisherSpecs[publisher.Name]

				if !ex {
					log.WithFields(log.Fields{
						"spec": publisher.Spec,
						"name": publisher.Name,
					}).Trace("New publisher")
					s.publisherSpecs[publisher.Name] = publisher
				} else {
					log.WithFields(log.Fields{
						"spec": publisher.Spec,
						"name": publisher.Name,
					}).Trace("Updated publisher")
					s.publisherSpecs[publisher.Name] = publisher
				}
			}
			for k := range s.publisherSpecs {
				found := false
				for _, publisher := range cfg.Publishers {
					if publisher.Name == k {
						found = true
					}
				}
				if found == false {
					delete(s.publisherSpecs, k)
					log.WithField("name", k).Trace("Deleted publisher")
				}
			}

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("Stopping publish worker")
			wg.Done()
			return
		}
	}
}
