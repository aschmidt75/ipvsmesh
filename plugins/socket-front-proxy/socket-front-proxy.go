package socketfrontproxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"reflect"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// Spec is the spec subpart of a service for the docker front proxy plugin
type Spec struct {
	MatchSocket MatchSocketSpec `yaml:"matchSocket"`
	//DynamicWeights []*DynamicWeightsSpec `yaml:"dynamicWeights,omitempty"`

	mu            sync.Mutex
	lastListeners []Listener
	procnetfile   string
}

// DynamicWeightsSpec associates a number of matchLabels with a concrete weight
// All containers matching the labels are given that weight.
type DynamicWeightsSpec struct {
	Weight      int               `yaml:"weight"`
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type MatchSocketSpec struct {
	Address  string    `yaml:"address"`
	Protocol string    `yaml:"protocol,omitempty"`
	Ports    PortsSpec `yaml:"ports"`
}

type PortsSpec struct {
	From uint16 `yaml:"from"`
	To   uint16 `yaml:"to"`
}

func (s *Spec) initialize() error {
	return nil
}

// Name returns the plugin name
func (s *Spec) Name() string {
	return "socketFrontProxy"
}

// HasDownwardInterface is true, plugin checks local docker containers for new ips
func (s *Spec) HasDownwardInterface() bool {
	return true
}

// Initialize the plugin
func (s *Spec) Initialize(globals *model.Globals) error {
	log.Trace("socket-front-proxy: initialize")

	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastListeners = []Listener{}

	if s.MatchSocket.Protocol == "udp" {
		s.procnetfile = "/proc/net/udp"
	}
	s.procnetfile = "/proc/net/tcp"

	if globals != nil {
		v, ex := globals.Settings["socketFrontProxy.procnet.file"]
		if ex {
			s.procnetfile = v
			log.WithField("procnetfile", v).Trace("Using different proc-net file")
		}
	}
	return nil
}

// GetDownwardData queries /proc/net/{tcp,udp} for listening sockets and
// returns all entries matching the MatchSocket entry
func (s *Spec) GetDownwardData() ([]model.DownwardBackendServer, error) {
	res := make([]model.DownwardBackendServer, 0)

	for _, listener := range s.lastListeners {
		//log.WithField("l", listener).Trace("listener")
		if listener.port >= s.MatchSocket.Ports.From && listener.port <= s.MatchSocket.Ports.To {

			ip, ipnet, err := net.ParseCIDR(s.MatchSocket.Address)
			if err != nil {
				log.WithField("a", s.MatchSocket.Address).Error("socket-front-proxy: Not in CIDR form, skipping")
				continue
			}
			log.WithFields(log.Fields{
				"ip":    ip,
				"ipnet": ipnet,
				"from":  s.MatchSocket.Address,
			}).Trace("Parsed")
			if ipnet.Contains(listener.ip) {
				a := fmt.Sprintf("%s:%d", listener.ip, listener.port)
				log.WithField("addr", a).Debug("socket-front-proxy: Matching ip/port")
				res = append(res, model.DownwardBackendServer{
					Address: a,
				})
			}
		}
	}

	return res, nil
}

// HasUpwardInterface is false, does not expose something
func (s *Spec) HasUpwardInterface() bool {
	return false
}

func (s *Spec) queryProcNetOnce(notChan chan struct{}) {
	var listeners []Listener

	listeners, err := ParseProcNetTcpUdpFromFile(s.procnetfile)
	if err != nil {
		log.WithField("err", err).Error("Unable to read from proc net file, skipping")
		return
	}
	if reflect.DeepEqual(s.lastListeners, listeners) == false {
		s.lastListeners = listeners
		log.Trace("socket-front-proxy: Found port update")
		notChan <- struct{}{}
	}

}

// RunNotificationLoop monitors /proc/net/{tcp,udp} for changes. Each matching event will trigger an update on notCh
func (s *Spec) RunNotificationLoop(notChan chan struct{}, quitChan chan struct{}) error {
	log.WithField("Name", s.Name()).Debug("socket-front-proxy: Starting notification loop")

	err := s.initialize()
	if err != nil {
		// do not start loop.
		return err
	}

	s.queryProcNetOnce(notChan)

	for {
		select {
		case <-time.After(1000 * time.Millisecond):
			s.queryProcNetOnce(notChan)
		case <-quitChan:
			log.WithField("Name", s.Name()).Debug("socket-front-proxy: Stopped notification loop")
			return nil
		}
	}
}

func (s *Spec) PushUpwardData(data model.UpwardData) error {
	return nil
}
