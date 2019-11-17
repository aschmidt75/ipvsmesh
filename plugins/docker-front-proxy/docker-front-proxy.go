package dockerfrontproxy

import (
	"context"
	"fmt"
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	client "github.com/docker/docker/client"
)

// Spec is the spec subpart of a service for the docker front proxy plugin
type Spec struct {
	MatchLabels map[string]string `yaml:"matchLabels"`

	mu           sync.Mutex
	dockerClient *client.Client
}

func (s *Spec) initialize() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dockerClient == nil {
		var err error
		s.dockerClient, err = client.NewEnvClient()
		if err != nil {
			log.WithField("err", err).Error("unable to create docker client")
		}
		log.WithField("docker", s.dockerClient).Debug("Docker Client")
	}
}

// Name returns the plugin name
func (s *Spec) Name() string {
	return "docker-front-proxy"
}

// HasDownwardInterface is true, plugin checks local docker containers for new ips
func (s *Spec) HasDownwardInterface() bool {
	return true
}

// GetDownwardData queries
func (s *Spec) GetDownwardData() ([]model.DownwardBackendServer, error) {

	s.initialize()

	ctx := context.Background()

	args := filters.NewArgs()
	for k, v := range s.MatchLabels {
		args.Add("label", fmt.Sprintf("%s=%s", k, v))
	}
	opts := types.ContainerListOptions{
		Filters: args,
	}
	containers, err := s.dockerClient.ContainerList(ctx, opts)
	if err != nil {
		log.WithField("err", err).Error("unable to query containers")
	}

	res := make([]model.DownwardBackendServer, len(containers))
	for idx, container := range containers {
		endpointIP := ""
		endpointPort := uint16(0)
		for _, port := range container.Ports {
			log.WithFields(log.Fields{
				"port": port,
			}).Trace("found port")
			endpointPort = port.PrivatePort

		}
		if container.NetworkSettings != nil {
			for name, endpoint := range container.NetworkSettings.Networks {
				log.WithFields(log.Fields{
					"net": name,
					"ip":  endpoint.IPAddress,
				}).Trace("found endpoint")
				endpointIP = endpoint.IPAddress
			}
		}

		res[idx] = model.DownwardBackendServer{Address: fmt.Sprintf("%s:%d", endpointIP, endpointPort)}
	}

	return res, nil
}

// HasUpwardInterface is false, does not expose something
func (s *Spec) HasUpwardInterface() bool {
	return false
}

// RunNotificationLoop connects to docker daemon and waits for
// container events. Each matching event will trigger an update on notCh
func (s *Spec) RunNotificationLoop(notChan chan struct{}) error {
	log.WithField("Name", s.Name()).Debug("Starting notification loop")

	s.initialize()

	ctx, _ := context.WithCancel(context.Background())

	// listen for container messages only, from containers with given labels.
	args := filters.NewArgs()
	args.Add("type", "container")
	for k, v := range s.MatchLabels {
		args.Add("label", fmt.Sprintf("%s=%s", k, v))
	}
	log.WithField("filters", args).Trace("looking for this docker events")

	msgs, errs := s.dockerClient.Events(ctx, types.EventsOptions{
		Filters: args,
	})

	for {
		select {
		case err := <-errs:
			log.WithField("err", err).Debug("docker client error")
		case msg := <-msgs:
			if msg.Action == "start" || msg.Action == "restart" || msg.Action == "stop" || msg.Action == "die" || msg.Action == "pause" || msg.Action == "unpause" {
				log.WithField("msg", msg).Trace("docker client message")
				notChan <- struct{}{}
			}
		case <-notChan:
			break
		}
	}
	log.WithField("Name", s.Name()).Debug("Stopped notification loop")
	return nil
}
