package daemon

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// IPVSApplierUpdateStruct is a message from a plugin with a downward api.
// The message affects a single service from the model and contains data updates.
type IPVSApplierUpdateStruct struct {
	serviceName string
	cfg         *model.IPVSMeshConfig
	service     *model.Service
	data        []model.DownwardBackendServer
}

// IPVSModelStruct ...
type IPVSModelStruct map[string]interface{}

// IPVSApplierChanType ...
type IPVSApplierChanType chan IPVSApplierUpdateStruct

// IPVSApplierWorker is a worker maintaining the recent
// ipvs configuration model. It receives downward api updates from
// a channel and merges them into the model, updating state using ipvsctl
type IPVSApplierWorker struct {
	StoppableByChan

	updateChan IPVSApplierChanType
	cfg        *model.IPVSMeshConfig
	services   map[string]IPVSApplierUpdateStruct
	mu         sync.Mutex
}

// NewIPVSApplierWorker creates an IPVS applier worker based on
// an update channel and the recent model
func NewIPVSApplierWorker(updateChan IPVSApplierChanType) *IPVSApplierWorker {
	sc := make(chan *sync.WaitGroup, 1)

	return &IPVSApplierWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		updateChan: updateChan,
		cfg:        nil,
		services:   make(map[string]IPVSApplierUpdateStruct, 5),
	}
}

// integrates an update from the downward api into the current overall model and
// produces an IPVS ctl conformant model
func (s *IPVSApplierWorker) integrateUpdate(u IPVSApplierUpdateStruct) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// this is the current model/config we're operating on
	s.cfg = u.cfg

	w := u.service.Weight
	if w == 0 {
		w = 1000
	}
	sched := u.service.SchedName
	if sched == "" {
		sched = "wrr"
	}
	forward := u.service.Forward
	if forward == "" {
		forward = "nat"
	}

	// copy update into own cached model
	s.services[u.serviceName] = u

	// recreate target
	var target IPVSModelStruct
	target = make(IPVSModelStruct, 5)

	// count services with non-empty destinations list
	numNonEmptyServices := 0
	for _, service := range s.services {
		if len(service.data) > 0 {
			numNonEmptyServices++
		}
	}

	tss := make([]interface{}, numNonEmptyServices)
	target["services"] = tss

	idx := 0
	for _, service := range s.services {
		if len(service.data) == 0 {
			continue
		}

		ts := make(map[string]interface{})
		tss[idx] = ts
		ts["address"] = u.service.Address
		ts["ipvsmesh.service.name"] = u.service.Name
		ts["ipvsmesh.service.type"] = u.service.Type
		ts["sched"] = sched

		td := make([]interface{}, len(service.data))
		ts["destinations"] = td
		for idx, downwardBackendServer := range service.data {
			tdd := make(map[string]interface{}, 3)
			td[idx] = tdd
			tdd["address"] = downwardBackendServer.Address
			tdd["forward"] = forward
			tdd["weight"] = w
			for k, v := range downwardBackendServer.AdditionalInfo {
				tdd[fmt.Sprintf("ipvsmesh.%s", k)] = v
			}
		}

		idx++
	}

	// ensure "destinations"

	return target, nil
}

func (s *IPVSApplierWorker) applyUpdate(target map[string]interface{}) error {
	//
	b, err := yaml.Marshal(target)
	if err != nil {
		return err
	}
	log.WithField("yaml", string(b)).Trace("Applying ipvsctl model")
	log.Debug("Applying ipvsctl model")

	execType := s.cfg.Globals.Ipvsctl.ExecType
	if execType == "" {
		execType = "exec-only"
	}
	fileName := s.cfg.Globals.Ipvsctl.Filename
	if fileName == "" {
		fileName = "/etc/ipvsmesh-ipvsctl.yaml"
	}
	ipvsctlPath := s.cfg.Globals.Ipvsctl.IpvsctlPath
	if ipvsctlPath == "" {
		ipvsctlPath = "ipvsctl" // must be in system path if no specific path given
	}

	log.WithFields(log.Fields{
		"type": execType,
		"file": fileName,
		"cmd":  ipvsctlPath,
	}).Debug("Applying with these settings...")
	switch execType {
	case "file-only":
		return ioutil.WriteFile(fileName, b, 0640)

	case "file-and-exec":
		err := ioutil.WriteFile(fileName, b, 0640)
		if err != nil {
			return err
		}
		ipvsctl := exec.Command(ipvsctlPath, "apply", "-f")
		return ipvsctl.Run()

	case "exec-only":
		// directly write into new process
		ipvsctl := exec.Command(ipvsctlPath, "apply")
		buffer := bytes.Buffer{}
		buffer.Write(b)

		ipvsctl.Stdout = os.Stdout
		ipvsctl.Stdin = &buffer
		ipvsctl.Stderr = os.Stderr

		return ipvsctl.Run()
	default:
		return fmt.Errorf("unknown executionType given: %s", execType)
	}
}

// Worker ...
func (s *IPVSApplierWorker) Worker() {
	log.Info("Starting IPVS applier...")
	for {
		select {
		case cfg := <-s.updateChan:
			// if serviceName is empty, flush all services from local cache map but do not apply this (empty) config
			if cfg.serviceName == "" {
				s.services = make(map[string]IPVSApplierUpdateStruct, 5)
				log.Debug("Flushing service cache after config refresh")
				break
			}

			log.WithField("cfg", cfg).Debug("Received new ipvs update")

			target, err := s.integrateUpdate(cfg)
			if err != nil {
				log.WithField("err", err).Error("Unable to integrate update")
			}

			err = s.applyUpdate(target)
			if err != nil {
				log.WithField("err", err).Error("Unable to apply update")
			}

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("Stopping IPVS Applier")

			<-time.After(1 * time.Second)
			wg.Done()
			return
		}
	}
}
