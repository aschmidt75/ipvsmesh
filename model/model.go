package model

// Service describes an IPVS service entry
type Service struct {
	Name    string                      `yaml:"name"`
	Address string                      `yaml:"address"`
	Type    string                      `yaml:"type"`
	Spec    map[interface{}]interface{} `yaml:"spec"`

	Plugin PluginSpec
}

// IPVSMeshConfig is the main confoguration structure
type IPVSMeshConfig struct {
	Services []*Service `yaml:"services,omitempty"`
}

//

// PluginSpec is a
type PluginSpec interface {

	// Name returns the name of the plugin
	Name() string

	// HasDownwardInterface returns true if this plugin can return
	// ip addresses of ipvs real/backend servers. It produces something
	// for us
	HasDownwardInterface() bool

	// RunNotificationLoop is a loop that pings the given channel
	// whenever the plugin has detected an update. It terminates
	// when it receives something on this channel
	RunNotificationLoop(notChan chan struct{}) error

	// GetDownwardData retrieves the data for real/backend servers
	GetDownwardData() ([]DownwardBackendServer, error)

	// HasUpwardInterface returns true if this plugin can deliver
	// data of the local ipvs table to other locations. We have something
	// that it can use (e.g. publish on a k/v store)
	HasUpwardInterface() bool
}

// DownwardBackendServer is a
type DownwardBackendServer struct {
	Address string
}
