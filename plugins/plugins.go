package plugins

import (
	"github.com/aschmidt75/ipvsmesh/model"
	dockerfrontproxy "github.com/aschmidt75/ipvsmesh/plugins/docker-front-proxy"
	"gopkg.in/yaml.v2"
)

// PluginSpec is a
type PluginSpec interface {

	// Name returns the name of the plugin
	Name() string

	// HasDownwardInterface returns true if this plugin can return
	// ip addresses of ipvs real/backend servers
	HasDownwardInterface() bool

	// GetDownwardData retrieves the data for real/backend servers
	GetDownwardData() ([]model.DownwardBackendServer, error)

	// HasUpwardInterface returns true if this plugin can deliver
	// data of the local ipvs table to other locations
	HasUpwardInterface() bool
}

// ReadPluginSpecByTypeString takes the spec part of a services and
// returns a plugin spec object
func ReadPluginSpecByTypeString(service *model.Service) (PluginSpec, error) {

	b, err := yaml.Marshal(service.Spec)
	if err != nil {
		return nil, err
	}

	var res PluginSpec
	if service.Type == "docker-front-proxy" {
		res = &dockerfrontproxy.Spec{}
	}

	err = yaml.Unmarshal(b, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
