package filepublisher

import (
	"io/ioutil"
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Spec is the spec subpart of a service for the etcd publisher plugin
type Spec struct {
	Output OutputSpec `yaml:"output"`

	mu sync.Mutex
	// etcdclient
}

// OutputSpec describes where the output file should be writte to,
// what its format should be (e.g. yaml) and the type of data (delta,data,full)
type OutputSpec struct {
	OutputFile   string `yaml:"outputFile"`
	OutputFormat string `yaml:"outputFormat,omitempty"`
	OutputType   string `yaml:"outputType,omitempty"`
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

	if s.Output.OutputType == "yaml" {
		b, err := yaml.Marshal(data.Update)
		if err != nil {
			return err
		}

		ioutil.WriteFile(s.Output.OutputFile, b, 0644)
	}

	return nil
}
