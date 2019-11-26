package daemon

import (
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	"github.com/aschmidt75/ipvsmesh/plugins"
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
func (s *PublisherhWorker) walkUpdate(upd PublisherUpdate) ([]*model.Publisher, error) {
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
		}).Trace("Found publishers for matchLabels")
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

func (s *PublisherhWorker) triggerPublishing(publishers []*model.Publisher) error {
	for _, publisher := range publishers {
		log.WithField("name", publisher.Name).Debug("Triggering publish update")

		if publisher.Plugin == nil {
			log.WithField("name", publisher.Name).Warn("Invalid plugin spec, skipping")
			continue
		}

		originService, ex := s.getFirstServiceByLabels(publisher.MatchLabels)
		if !ex {
			log.WithField("name", publisher.Name).Warn("No services match MatchLabels, skipping")
			continue
		}

		// Forward to plugin
		ud := model.UpwardData{
			Address:         originService.Address,
			ServiceName:     originService.Name,
			OriginService:   originService,
			TargetPublisher: publisher,
		}
		if err := publisher.Plugin.PushUpwardData(ud); err != nil {
			log.WithFields(log.Fields{
				"err":         err,
				"serviceName": ud.ServiceName,
			}).Error("Unable to trigger publishing")
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

			publishers, err := s.walkUpdate(upd)
			if err != nil {
				log.WithField("err", err).Error("Unable to process publisher update")
				continue
			}

			err = s.triggerPublishing(publishers)
			if err != nil {
				log.WithField("err", err).Error("Unable to publish update")
				continue
			}

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
