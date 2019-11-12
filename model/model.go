package model

// Service describes an IPVS service entry
type Service struct {
	Name    string                      `yaml:"name"`
	Address string                      `yaml:"address"`
	Type    string                      `yaml:"type"`
	Spec    map[interface{}]interface{} `yaml:"spec"`
}

// IPVSMeshConfig is the main confoguration structure
type IPVSMeshConfig struct {
	Services []*Service `yaml:"services,omitempty"`
}

type DownwardBackendServer struct {
	Address string
}
