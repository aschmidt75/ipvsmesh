package dockerfrontproxy

import (
	"context"
	"fmt"
	"sync"
	"time"

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

func (s *Spec) initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dockerClient == nil {
		var err error
		s.dockerClient, err = client.NewEnvClient()
		if err != nil {
			log.WithField("err", err).Error("docker-front-proxy: unable to create docker client")
			return err
		}
		log.WithField("docker", s.dockerClient).Trace("docker-front-proxy: Docker Client")
	}
	return nil
}

// Name returns the plugin name
func (s *Spec) Name() string {
	return "dockerFrontProxy"
}

// HasDownwardInterface is true, plugin checks local docker containers for new ips
func (s *Spec) HasDownwardInterface() bool {
	return true
}

// GetDownwardData queries docker containers by spec and returns
// a list of ip address/port number endpoints
func (s *Spec) GetDownwardData() ([]model.DownwardBackendServer, error) {

	err := s.initialize()
	if err != nil {
		return make([]model.DownwardBackendServer, 0), err
	}

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
		log.WithField("err", err).Error("docker-front-proxy: unable to query containers")
	}

	numRunning := 0
	for _, container := range containers {
		if container.State == "running" {
			numRunning++
		}
	}

	res := make([]model.DownwardBackendServer, numRunning)
	idx := 0
	for _, container := range containers {
		if container.State != "running" {
			continue
		}
		endpointIP := ""
		endpointPort := uint16(0)
		for _, port := range container.Ports {
			endpointPort = port.PrivatePort

		}
		if container.NetworkSettings != nil {
			for _, endpoint := range container.NetworkSettings.Networks {
				endpointIP = endpoint.IPAddress
			}
		}

		log.WithFields(log.Fields{
			"id":     container.ID,
			"state":  container.State,
			"status": container.Status,
			"ip":     endpointIP,
			"port":   endpointPort,
		}).Trace("docker-front-proxy: found matching container")

		addData := make(map[string]string)
		addData["container.id"] = container.ID
		if len(container.Names) > 0 {
			addData["container.name"] = container.Names[0]
		}

		res[idx] = model.DownwardBackendServer{
			Address:        fmt.Sprintf("%s:%d", endpointIP, endpointPort),
			AdditionalInfo: addData,
		}
		idx++
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
	log.WithField("Name", s.Name()).Debug("docker-front-proxy: Starting notification loop")

	err := s.initialize()
	if err != nil {
		// do not start loop.
		return err
	}

	ctx, _ := context.WithCancel(context.Background())

	// listen for container messages only, from containers with given labels.
	args := filters.NewArgs()
	args.Add("type", "container")
	for k, v := range s.MatchLabels {
		args.Add("label", fmt.Sprintf("%s=%s", k, v))
	}
	log.WithField("filters", args).Trace("docker-front-proxy: watching docker events")

	msgs, errs := s.dockerClient.Events(ctx, types.EventsOptions{
		Filters: args,
	})

	for {
		select {
		case err := <-errs:
			log.WithField("err", err).Error("docker-front-proxy: docker client error")
			// TODO: try reconnect?
			return nil
		case msg := <-msgs:
			if msg.Action == "start" || msg.Action == "restart" || msg.Action == "stop" || msg.Action == "die" || msg.Action == "pause" || msg.Action == "unpause" {
				log.WithField("msg", msg).Trace("docker-front-proxy: docker client message")
				go func() {
					<-time.After(50 * time.Millisecond)
					notChan <- struct{}{}
				}()
			}
		case <-notChan:
			log.WithField("Name", s.Name()).Debug("docker-front-proxy: Stopped notification loop")
			return nil
		}
	}
}

func (s *Spec) PushUpwardData(data model.UpwardData) error {
	return nil
}
