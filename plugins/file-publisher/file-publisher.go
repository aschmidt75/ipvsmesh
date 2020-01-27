package filepublisher

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Spec is the spec subpart of a service for the etcd publisher plugin
type Spec struct {
	Output OutputSpec `yaml:"output"`

	mu sync.Mutex
}

// OutputSpec describes where the output file should be writte to,
// what its format should be (e.g. yaml) and the type of data (deltaonly,dataonly,full)
type OutputSpec struct {
	OutputFile     string `yaml:"outputFile"`
	OutputFormat   string `yaml:"outputFormat,omitempty"`
	OutputType     string `yaml:"outputType,omitempty"`
	OutputFileMode *int32 `yaml:"outputFileMode,omitempty"`
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

func (s *Spec) PushUpwardData(dataT model.UpwardData) error {
	log.WithField("data", dataT).Debug("PushUpwardData ->")

	// clean
	data := model.UpwardData{
		TargetPublisher: dataT.TargetPublisher,
		Update: model.EndpointUpdate{
			Delta:          dataT.Update.Delta,
			Endpoints:      dataT.Update.Endpoints,
			AdditionalInfo: dataT.Update.AdditionalInfo,
		},
	}
	if s.Output.OutputFormat == "deltaonly" {
		data.Update.Endpoints = nil
	}
	if s.Output.OutputFormat == "dataonly" {
		data.Update.Delta = nil
	}

	// default file mode
	var ofm os.FileMode = 0644
	if s.Output.OutputFileMode != nil {
		ofm = os.FileMode(*s.Output.OutputFileMode)
	}

	// write to file
	if s.Output.OutputType == "yaml" {
		b, err := yaml.Marshal(data.Update)
		if err != nil {
			return err
		}

		ioutil.WriteFile(s.Output.OutputFile, b, ofm)
	}

	if s.Output.OutputType == "json" {
		b, err := json.Marshal(data.Update)
		if err != nil {
			return err
		}

		ioutil.WriteFile(s.Output.OutputFile, b, ofm)
	}
	return nil
}
