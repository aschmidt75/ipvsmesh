package model

// Service describes an IPVS service entry
type Service struct {
	Name      string `yaml:"name"`
	Address   string `yaml:"address"`
	Type      string `yaml:"type"`
	SchedName string `yaml:"sched,omitempty"`   // default: wrr
	Weight    int    `yaml:"weight,omitempty"`  // default: 1000
	Forward   string `yaml:"forward,omitempty"` // default: nat

	// Spec is the specification for a service. See
	// plugins/* for concrete Spec structs
	Spec map[interface{}]interface{} `yaml:"spec"`

	Plugin PluginSpec
}

// Globals contains global configuration entries for all ipvsmesh
type Globals struct {
	Ipvsctl IpvsctlConfig `yaml:"ipvsctl,omitempty"`
}

// IpvsctlConfig describes the mode-of-operation for applying
// updates via ipvsctl
type IpvsctlConfig struct {
	ExecType    string `yaml:"executionType,omitempty"` // file-only, file-and-exec, exec-only, direct
	Filename    string `yaml:"file,omitempty"`
	IpvsctlPath string `yaml:"ipvsctlPath,omitempty"`
}

// IPVSMeshConfig is the main confoguration structure
type IPVSMeshConfig struct {
	Globals  Globals    `yaml:"globals,omitempty"`
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

// DownwardBackendServer contains all data regarding a concrete
// endpoint, with an address string suitable for ipvsctl's model.
// It may contain additional data (e.g. ids) in a map.
type DownwardBackendServer struct {
	Address        string
	Weight         int
	AdditionalInfo map[string]string
}
