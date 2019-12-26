package daemon

import (
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	"github.com/aschmidt75/ipvsmesh/plugins"
	log "github.com/sirupsen/logrus"
)

// PublisherUpdate is a message from the ipvs applier indicating
// that an endpoint has changed. It is the raw data structure for
// the ipvsctl model
type PublisherUpdate struct {
	data map[string]interface{}
}

// PublisherUpdateChanType is for backend updates
type PublisherUpdateChanType chan PublisherUpdate

// PublisherConfigUpdateChanType is for configuration updates
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

	// remember what services (by name) we have published.
	publishedServices map[string]bool
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

		publisherSpecs:    make(map[string]*model.Publisher, 5),
		publishedServices: make(map[string]bool, 5),
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

func (s *PublisherhWorker) getFirstServiceByLabels(matchLabels map[string]string) (*model.Service, bool) {
	for _, service := range s.cfg.Services {
		found := true
		for k, v := range matchLabels {
			vv, ex := service.Labels[k]
			if !ex || v != vv {
				found = false
			}
		}
		if found {
			return service, true
		}
	}
	return nil, false
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
			}).Trace("Found publisher by labels")
			res = append(res, publisher)
		}
	}

	return res
}

// walks over a PublisherUpdate, looks up affected
// services/publishers and returns a list of them
func (s *PublisherhWorker) walkUpdate_DELETE(upd PublisherUpdate) ([]*model.Publisher, error) {
	servicesRaw, ex := upd.data["services"]
	if !ex {
		// no services there. Notify all publishers to take down existings endpoints
		log.Debug("PublisherWorker: No services")
		return make([]*model.Publisher, 0), nil
	}

	services := servicesRaw.([]interface{})

	m := make(map[string]*model.Publisher)

	for _, serviceRaw := range services {
		service := serviceRaw.(map[string]interface{})

		// find out with ipvsmesh service is behind this ipvsctl service
		serviceName, ex := service["ipvsmesh.service.name"].(string)
		if !ex {
			log.WithField("serviceName", serviceName).Error("PublisherWorker: Internal error, unable to track ipvsctl service.")
			continue
		}

		// look up labels of service
		labels, found := s.getLabelsByServiceName(serviceName)
		if !found {
			log.WithField("serviceName", serviceName).Error("PublisherWorker: Internal error, unable to find ipvsctl service (2).")
			continue
		}
		log.WithField("labels", labels).Trace("PublisherWorker: Labels for service name")

		// look up publishers with these labels
		publishers := s.findPublishersByLabels(labels)
		log.WithFields(log.Fields{
			"num":         len(publishers),
			"serviceName": serviceName,
		}).Trace("PublisherWorker: Found publishers for matchLabels")
		for _, p := range publishers {
			m[p.Name] = p
		}
	}

	res := make([]*model.Publisher, len(m))
	idx := 0
	for _, v := range m {
		res[idx] = v
		idx++
	}

	return res, nil
}

func (s *PublisherhWorker) triggerPublishing_DELETE(publishers []*model.Publisher) error {
	for _, publisher := range publishers {
		log.WithField("name", publisher.Name).Debug("PublisherWorker: Triggering publish update")

		if publisher.Plugin == nil {
			log.WithField("name", publisher.Name).Warn("PublisherWorker: Invalid plugin spec, skipping")
			continue
		}

		originService, ex := s.getFirstServiceByLabels(publisher.MatchLabels)
		if !ex {
			log.WithField("name", publisher.Name).Warn("PublisherWorker: No services match MatchLabels, skipping")
			continue
		}

		// Forward to plugin
		ud := model.UpwardData{
			Address:         originService.Address,
			ServiceName:     originService.Name,
			OriginService:   originService,
			TargetPublisher: publisher,
		}
		err := publisher.Plugin.PushUpwardData(ud)
		if err != nil {
			log.WithFields(log.Fields{
				"err":         err,
				"serviceName": ud.ServiceName,
			}).Error("PublisherWorker: Unable to trigger publishing")
		} else {
			s.publishedServices[ud.ServiceName] = true
		}

	}
	return nil
}

// Worker checks downward notifications
func (s *PublisherhWorker) Worker() {
	log.Info("Starting publish worker...")

	for {
		select {
		case upd := <-s.updateCh:
			log.WithField("upd", upd).Debug("PublisherWorker: got backend update for publishing")
		/*
			publishers, err := s.walkUpdate(upd)
			if err != nil {
				log.WithField("err", err).Error("PublisherWorker: Unable to process publisher update")
				continue
			}

			err = s.triggerPublishing(publishers)
			if err != nil {
				log.WithField("err", err).Error("PublisherWorker: Unable to publish update")
				continue
			}
		*/

		// store new updated model along side previous model. Split/map by service names

		// Walk all publishers. Locate services by publishers
		// compare new and previous model.
		// 1. "new" contains a service and "previous" does not. Publish new endpoint
		// 2. "new" contains a service and "previous" does also. Do nothing.
		// 3. "new" is missing a service which "previous" contained. Remove endpoint

		// "new" is recent now

		case cfg := <-s.configUpdateCh:
			log.WithField("numPublishers", len(cfg.Publishers)).Debug("PublisherWorker: got config update")

			s.cfg = cfg

			for _, publisher := range cfg.Publishers {
				// load plugin spec
				if publisher.Plugin == nil {
					var err error
					publisher.Plugin, err = plugins.ReadPublisherPluginSpecByTypeString(publisher)
					if err != nil {
						log.WithField("err", err).Error("unable to parse spec for publisher")
					}
				}

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

				// TODO: since we have a new or updated published we need to
				// walk the services and see what we need to update.
				// This could mean that updating a publisher is really a deletion and
				// re-creation.
			}
			for k := range s.publisherSpecs {
				found := false
				for _, publisher := range cfg.Publishers {
					if publisher.Name == k {
						found = true
					}
				}
				if found == false {

					// TODO: deleting a publisher means we have to inform the publisher
					// so that it removes everything it has currently published. Because
					// it will not be valid any more.

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
