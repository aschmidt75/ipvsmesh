package filepublisher

import (
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
)

// Spec is the spec subpart of a service for the etcd publisher plugin
type Spec struct {
	MatchLabels map[string]string `yaml:"matchLabels"`

	OutputFile string `yaml:"outputFile"`
	OutputType string `yaml:"outputType,omitempty"`

	mu sync.Mutex
	// etcdclient
}

func (s *Spec) initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	//	if s.etcdclient == nil {
	//	}
	return nil
}

// Name returns the plugin name
func (s *Spec) Name() string {
	return "filePublisher"
}

// Initialize the plugin
func (s *Spec) Initialize(globals *model.Globals) error {
	return nil
}

func (s *Spec) HasDownwardInterface() bool {
	return false
}

func (s *Spec) RunNotificationLoop(notChan chan struct{}, quitChan chan struct{}) error {
	return nil
}

func (s *Spec) GetDownwardData() ([]model.DownwardBackendServer, error) {
	return []model.DownwardBackendServer{}, nil
}

func (s *Spec) HasUpwardInterface() bool {
	return true
}

func (s *Spec) PushUpwardData(data model.UpwardData) error {
	log.WithField("data", data).Debug("PushUpwardData ->")

	// write to file

	return nil
}
