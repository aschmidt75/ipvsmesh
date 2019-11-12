package dockerfrontproxy

import (
	"github.com/aschmidt75/ipvsmesh/model"
)

// Spec is the spec subpart of a service for the docker front proxy plugin
type Spec struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

// Name returns the plugin name
func (s Spec) Name() string {
	return "docker-front-proxy"
}

// HasDownwardInterface is true, plugin checks local docker containers for new ips
func (s Spec) HasDownwardInterface() bool {
	return true
}

// GetDownwardData queries
func (s Spec) GetDownwardData() ([]model.DownwardBackendServer, error) {
	res := make([]model.DownwardBackendServer, 2)
	res[0] = model.DownwardBackendServer{Address: "1.2.3.4:80"}
	res[1] = model.DownwardBackendServer{Address: "1.2.3.4:81"}
	return res, nil
}

// HasUpwardInterface is false, does not expose something
func (s Spec) HasUpwardInterface() bool {
	return false
}
