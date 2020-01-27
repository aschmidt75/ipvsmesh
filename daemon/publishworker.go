package daemon

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aschmidt75/ipvsmesh/model"
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

	// remember last process publisher update to be able
	// to compare it with the recent one and find the diff.
	last, recent PublisherUpdate

	onceFlag bool
	onceCh   chan struct{}
}

// NewPublisherWorker creates a worker for publish notifications
func NewPublisherWorker(updateChan PublisherUpdateChanType, configUpdateCh PublisherConfigUpdateChanType, onceFlag bool, onceCh chan struct{}) *PublisherhWorker {
	sc := make(chan *sync.WaitGroup, 1)
	return &PublisherhWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		updateCh:       updateChan,
		configUpdateCh: configUpdateCh,
		onceFlag:       onceFlag,
		onceCh:         onceCh,

		publisherSpecs: make(map[string]*model.Publisher, 5),
		last:           PublisherUpdate{data: make(map[string]interface{}, 0)},
		recent:         PublisherUpdate{data: make(map[string]interface{}, 0)},
	}
}

// given a service name, this returns the labels of the given service
func (s *PublisherhWorker) getLabelsByServiceName(name string) (map[string]string, bool) {
	for _, service := range s.cfg.Services {
		if service.Name == name {
			return service.Labels, true
		}
	}
	return make(map[string]string, 0), false
}

func (s *PublisherhWorker) getFirstServiceByMatchLabels(matchLabels map[string]string) (*model.Service, bool) {
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

func (s *PublisherhWorker) getServicesByMatchLabels(matchLabels map[string]string) []*model.Service {
	res := make([]*model.Service, 0)
	if len(matchLabels) == 0 {
		return res
	}

	for _, service := range s.cfg.Services {
		found := true
		for k, v := range matchLabels {
			vv, ex := service.Labels[k]
			if !ex || v != vv {
				found = false
			}
		}
		if found {
			log.WithFields(log.Fields{
				"ml": matchLabels,
				"sl": service.Labels,
				"sn": service.Name,
			}).Trace("Found matching service")
			res = append(res, service)
		}
	}
	return res
}

func (s *PublisherhWorker) processUpdateForServiceAndPublisher(service *model.Service, p *model.Publisher, eu model.EndpointUpdate) {

	// from given publisher, filter all services matching p.MatchLabels, re-org in map
	matchingServices := make(map[string]*model.Service, len(s.cfg.Services))
	for _, matchingService := range s.getServicesByMatchLabels(p.MatchLabels) {
		matchingServices[matchingService.Name] = matchingService
	}
	log.WithField("matchingSvcs", matchingServices).Tracef("PublisherWorker: Done matching services for publisher %s", p.Name)

	// compare new and previous model regarding MatchLabels given service

	lastServicesRaw, ex := s.last.data["services"]
	if !ex {
		lastServicesRaw = make([]interface{}, 0)
	}
	lastServices := lastServicesRaw.([]interface{})

	recentServicesRaw, ex := s.recent.data["services"]
	if !ex {
		recentServicesRaw = make([]interface{}, 0)
	}
	recentServices := recentServicesRaw.([]interface{})

	// walk the "recent" list.
	for _, recentServiceRaw := range recentServices {
		recentService := recentServiceRaw.(map[string]interface{})
		recentAddress := recentService["address"]

		recentAddressStr := recentAddress.(string)

		// find out with ipvsmesh service is behind this ipvsctl service
		recentServiceName, ex := recentService["ipvsmesh.service.name"].(string)
		if !ex {
			continue
		}
		// check match labels. if we cannot find it, skip it because its not covered by matchLabels
		_, ex = matchingServices[recentServiceName]
		if !ex {
			continue
		}

		*eu.Endpoints = append(*eu.Endpoints, recentAddressStr)

		// try to find this address in lastServices
		found := false
		for _, lastServiceRaw := range lastServices {
			lastService := lastServiceRaw.(map[string]interface{})
			lastAddress := lastService["address"]

			lastServiceName, ex := lastService["ipvsmesh.service.name"].(string)
			if !ex {
				continue
			}
			_, ex = matchingServices[lastServiceName]
			if !ex {
				continue
			}

			if recentAddress == lastAddress {
				found = true
			}
		}
		if !found {
			// 1. "recent" contains a service and "last" does not. This one is new, publish new endpoint
			log.WithFields(log.Fields{
				"svcname": recentServiceName,
				"a":       recentAddress,
			}).Debug("PublisherWorker: New endpoint appeared.")
			*eu.Delta = append(*eu.Delta, model.EndpointDelta{
				ChangeType:     "appeared",
				Address:        recentAddressStr,
				AdditionalInfo: fmt.Sprintf("fromService=%s", recentServiceName),
			})
		} else {
			// 2. "recent" contains a service and "last" does also. skip
		}

	}

	// Find missing ones. Walk the "last" list.
	for _, lastServiceRaw := range lastServices {
		lastService := lastServiceRaw.(map[string]interface{})
		lastAddress := lastService["address"]

		lastServiceName, ex := lastService["ipvsmesh.service.name"].(string)
		if !ex {
			continue
		}
		_, ex = matchingServices[lastServiceName]
		if !ex {
			continue
		}

		// try to find this address in recentServices
		found := false
		for _, recentServiceRaw := range recentServices {
			recentService := recentServiceRaw.(map[string]interface{})
			recentAddress := recentService["address"]

			// find out with ipvsmesh service is behind this ipvsctl service
			recentServiceName, ex := recentService["ipvsmesh.service.name"].(string)
			if !ex {
				continue
			}
			// check match labels. if we cannot find it, skip it because its not covered by matchLabels
			_, ex = matchingServices[recentServiceName]
			if !ex {
				continue
			}

			if recentAddress == lastAddress {
				found = true
			}
		}
		if !found {
			// 1. "last" contained a service and now "recent" does not. This one has vanished, remove endpoint
			log.WithFields(log.Fields{
				"svcname": lastServiceName,
				"a":       lastAddress,
			}).Debug("PublisherWorker: Endpoint vanished.")
			*eu.Delta = append(*eu.Delta, model.EndpointDelta{
				ChangeType:     "vanished",
				Address:        lastAddress.(string),
				AdditionalInfo: fmt.Sprintf("fromService=%s", lastServiceName),
			})
		} else {
			// 2. "last" contains a service and "last" does also. We already found that one above, skip.
		}
	}

}

func (s *PublisherhWorker) processUpdateForPublisher(upd PublisherUpdate, p *model.Publisher) (model.EndpointUpdate, error) {
	delta := make([]model.EndpointDelta, 0)
	endpoints := make([]string, 0)
	eu := model.EndpointUpdate{
		Timestamp: fmt.Sprintf("%d", time.Now().UnixNano()),
		Delta:     &delta,
		Endpoints: &endpoints,
	}

	log.WithField("pml", p.MatchLabels).Trace("PublisherWorker: processUpdateForPublisher")
	log.Tracef("%#v", p)

	// Locate services by publishers, via MatchLabels
	services := s.getServicesByMatchLabels(p.MatchLabels)
	for _, service := range services {
		s.processUpdateForServiceAndPublisher(service, p, eu)

	}

	return eu, nil
}

func (s *PublisherhWorker) processUpdateForPublishers(upd PublisherUpdate) error {
	var publisherErr error
	for name, publisher := range s.publisherSpecs {
		eus, err := s.processUpdateForPublisher(upd, publisher)
		if err != nil {
			log.WithField("pname", name).Error(err)
			publisherErr = errors.New("some publishers returned errors for this update")
		}
		//
		log.WithField("data", eus).Trace("PublisherWorker: EndpointUpdate")

		// did something change, do we have a delta?
		if eus.Delta != nil && len(*eus.Delta) > 0 {
			// yes, push to the publisher to do sth with it
			publisher.Plugin.PushUpwardData(model.UpwardData{
				Update:          eus,
				TargetPublisher: publisher,
			})
		}
	}

	return publisherErr
}

// Worker checks downward notifications
func (s *PublisherhWorker) Worker() {
	log.Info("Starting publish worker...")

	for {
		select {
		case upd := <-s.updateCh:
			log.WithField("upd", upd).Debug("PublisherWorker: got backend update for publishing")

			// store new updated model along side previous model. Split/map by service names
			s.recent = upd

			// Walk all publishers, process update
			if err := s.processUpdateForPublishers(upd); err != nil {
				log.Error(err)
			}
			// "last" is recent now
			s.last = s.recent

			if s.onceFlag {
				log.Info("PublisherWorker: Stopping due to --once")
				s.onceCh <- struct{}{}
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
					}).Trace("PublisherWorker: New publisher")
					s.publisherSpecs[publisher.Name] = publisher
				} else {
					log.WithFields(log.Fields{
						"spec": publisher.Spec,
						"name": publisher.Name,
					}).Trace("PublisherWorker: Updated publisher")
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
					log.WithField("name", k).Trace("PublisherWorker: Deleted publisher")
				}
			}

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("Stopping publish worker")
			wg.Done()
			return
		}
	}
}
