package model

// Service describes an IPVS service entry
type Service struct {
	// Name of a service
	Name string `yaml:"name"`

	// ipvsctl-style address, e.g. tcp://10.0.0.1:8000
	Address string `yaml:"address"`

	// Type of this service, in terms of plugin types
	Type string `yaml:"type"`

	// ipvsctl-style scheduler name, default: wrr
	SchedName string `yaml:"sched,omitempty"`

	// ipvsctl-style initial weight, default: 1000
	Weight int `yaml:"weight,omitempty"`

	// ipvsctl-style type of forward: nat, direct, tunnel
	Forward string `yaml:"forward,omitempty"` // default: nat

	// Additional labels to target this service
	Labels map[string]string `yaml:"labels,omitempty"`

	// Spec is the specification for a service. See
	// plugins/* for concrete Spec structs
	Spec map[interface{}]interface{} `yaml:"spec"`

	Plugin PluginSpec
}

// Publisher is a construct to watch services for updates and
// propagate them further.
type Publisher struct {
	// Name of a publisher
	Name string `yaml:"name"`

	// Type of this publisher, in terms of plugin types
	Type string `yaml:"type"`

	// Additional labels to target this publisher
	Labels map[string]string `yaml:"labels,omitempty"`

	// MatchLabels indicate what service this publisher should watch
	MatchLabels map[string]string `yaml:"matchLabels"`

	// Spec is the specification for a publisher. See
	// plugins/* for concrete Spec structs
	Spec map[interface{}]interface{} `yaml:"spec"`

	Plugin PluginSpec
}

// Globals contains global configuration entries for all ipvsmesh
type Globals struct {
	Ipvsctl IpvsctlConfig            `yaml:"ipvsctl,omitempty"`
	Config  map[string]ConfigProfile `yaml:"configProfiles,omitempty"`
}

// IpvsctlConfig describes the mode-of-operation for applying
// updates via ipvsctl
type IpvsctlConfig struct {
	ExecType    string `yaml:"executionType,omitempty"` // file-only, file-and-exec, exec-only, direct
	Filename    string `yaml:"file,omitempty"`
	IpvsctlPath string `yaml:"ipvsctlPath,omitempty"`
}

// ConfigProfile defines configuration to an external source or
// destination, e.g. docker daemon or etcd endpoint
type ConfigProfile struct {
	URL string `yaml:"url"`
}

// IPVSMeshConfig is the main confoguration structure. It contains
// Global definitions, a set of services and a set of publishers.
// Although this is not checked, a publisher without services does
// not make sense.
type IPVSMeshConfig struct {
	Globals    Globals      `yaml:"globals,omitempty"`
	Services   []*Service   `yaml:"services,omitempty"`
	Publishers []*Publisher `yaml:"publishers,omitempty"`
}

//

// PluginSpec is the specification for a plugin. It can
// have a downward api to get updates from others to be incorporated
// into the local ipvs config (e.g. docker containers), as well as an
// upward api to publish local ipvs endpoints to others (e.g. k/v stores)
type PluginSpec interface {

	// Name returns the name of the plugin
	Name() string

	// HasDownwardInterface returns true if this plugin can return
	// ip addresses of ipvs real/backend servers. It produces something
	// for us
	HasDownwardInterface() bool

	// RunNotificationLoop is a loop that pings the given channel
	// whenever the plugin has detected an update. It terminates
	// when it receives something on this channel.
	RunNotificationLoop(notChan chan struct{}) error

	// GetDownwardData retrieves the data for real/backend servers
	GetDownwardData() ([]DownwardBackendServer, error)

	// HasUpwardInterface returns true if this plugin can deliver
	// data of the local ipvs table to other locations. We have something
	// that it can use (e.g. to be published to a k/v store)
	HasUpwardInterface() bool

	// PushUpwardData is used to notify others upward about
	// changes in this model
	PushUpwardData(data UpwardData) error
}

// DownwardBackendServer contains all data regarding a concrete
// endpoint, with an address string suitable for ipvsctl's model.
// It may contain additional data (e.g. ids) in a map.
type DownwardBackendServer struct {
	// Address is an endpoint spec that can be used to
	// feed ipvsctl with it
	Address string

	// AdditionalInfo contains metadata such as the container id
	AdditionalInfo map[string]string
}

// UpwardData contains an endpoint in form of an ipvs address
type UpwardData struct {
	Address     string
	ServiceName string

	OriginService   *Service
	TargetPublisher *Publisher
}

// IPVSModelStruct is a generic data/map struct for storing
// an ipvsctl model
type IPVSModelStruct map[string]interface{}
