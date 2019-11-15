package plugins

import (
	"github.com/aschmidt75/ipvsmesh/model"
	dockerfrontproxy "github.com/aschmidt75/ipvsmesh/plugins/docker-front-proxy"
	"gopkg.in/yaml.v2"
)

// ReadPluginSpecByTypeString takes the spec part of a services and
// returns a plugin spec object
func ReadPluginSpecByTypeString(service *model.Service) (model.PluginSpec, error) {

	b, err := yaml.Marshal(service.Spec)
	if err != nil {
		return nil, err
	}

	var res model.PluginSpec
	if service.Type == "docker-front-proxy" {
		res = &dockerfrontproxy.Spec{}
	}

	err = yaml.Unmarshal(b, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
