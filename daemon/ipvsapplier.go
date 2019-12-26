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
	cfg         *model.IPVSMeshConfig
	serviceName string
	service     *model.Service
	data        []model.DownwardBackendServer
}

// IPVSApplierChanType ...
type IPVSApplierChanType chan IPVSApplierUpdateStruct

// IPVSApplierWorker is a worker maintaining the recent
// ipvs configuration model. It receives downward api updates from
// a channel and merges them into the model, updating state using ipvsctl
type IPVSApplierWorker struct {
	StoppableByChan

	updateChan          IPVSApplierChanType
	publisherUpdateChan PublisherUpdateChanType

	cfg *model.IPVSMeshConfig

	// remember all updates we received
	services map[string]IPVSApplierUpdateStruct
	mu       sync.Mutex
}

// NewIPVSApplierWorker creates an IPVS applier worker based on
// an update channel and the recent model
func NewIPVSApplierWorker(updateChan IPVSApplierChanType, publisherUpdateChan PublisherUpdateChanType) *IPVSApplierWorker {
	sc := make(chan *sync.WaitGroup, 1)

	return &IPVSApplierWorker{
		StoppableByChan: StoppableByChan{
			StopChan: &sc,
		},
		updateChan:          updateChan,
		publisherUpdateChan: publisherUpdateChan,
		cfg:                 nil,
		services:            make(map[string]IPVSApplierUpdateStruct, 5),
	}
}

// integrates an update from the downward api into the current overall model and
// produces an IPVS ctl conformant model
func (s *IPVSApplierWorker) integrateUpdate(u IPVSApplierUpdateStruct) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// this is the current model/config we're operating on
	s.cfg = u.cfg

	// fill in sane defaults. TODO: Refactor to global defaults struct
	w := u.service.Weight
	if w == 0 {
		w = 1000 // TODO: Defaults
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

	// recreate target. IPVSModelStruct is map[string]interface{}, so
	// here we're creating maps on the fly as the basis for a ipvsctl yaml file.
	var target model.IPVSModelStruct
	target = make(model.IPVSModelStruct, 5)

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
			// adjust weight in case of dynamic weights
			if downwardBackendServer.Weight >= 0 {
				w = downwardBackendServer.Weight
			}

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

	return target, nil
}

// applyUpdate takes an ipvsctl-conformant im-memory struct and passes
// it on to ipvsctl to be activated. This can be done in different ways.
func (s *IPVSApplierWorker) applyUpdate(target map[string]interface{}) error {
	//
	b, err := yaml.Marshal(target)
	if err != nil {
		return err
	}
	log.WithField("yaml", string(b)).Trace("ipvsapplier: Applying ipvsctl model")
	log.Debug("ipvsapplier: Applying ipvsctl model")

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
	}).Debug("ipvsapplier: Applying with these settings...")
	switch execType {
	case "file-only":
		// just write the file and be done
		return ioutil.WriteFile(fileName, b, 0640)

	case "file-and-exec":
		// write the file and run ipvsctl apply
		err := ioutil.WriteFile(fileName, b, 0640)
		if err != nil {
			return err
		}
		ipvsctl := exec.Command(ipvsctlPath, "apply", "-f")
		return ipvsctl.Run()

	case "exec-only":
		// execute ipvsctl apply from stdin, directly write into new process
		ipvsctl := exec.Command(ipvsctlPath, "apply")
		buffer := bytes.Buffer{}
		buffer.Write(b)

		ipvsctl.Stdout = os.Stdout
		ipvsctl.Stdin = &buffer
		ipvsctl.Stderr = os.Stderr

		return ipvsctl.Run()
	default:
		return fmt.Errorf("ipvsapplier: unknown executionType given: %s", execType)
	}
}

// Worker ...
func (s *IPVSApplierWorker) Worker() {
	log.Info("ipvsapplier: Starting IPVS applier...")
	for {
		select {
		case cfg := <-s.updateChan:
			// if serviceName is empty, flush all services from local cache map but do not apply this (empty) config
			if cfg.serviceName == "" {
				s.services = make(map[string]IPVSApplierUpdateStruct, 5)
				log.Debug("ipvsapplier: Flushing service cache after config refresh")
				break
			}

			log.WithField("cfg", cfg).Debug("ipvsapplier: Received new ipvs update")

			target, err := s.integrateUpdate(cfg)
			if err != nil {
				log.WithField("err", err).Error("ipvsapplier: Unable to integrate update")
			}

			err = s.applyUpdate(target)
			if err != nil {
				log.WithField("err", err).Error("ipvsapplier: Unable to apply update")
			}

			// Notify publishers about the change, so they can propagate it further
			s.publisherUpdateChan <- PublisherUpdate{
				data: target,
			}

		case wg := <-*s.StoppableByChan.StopChan:
			log.Info("ipvsapplier: Stopping IPVS Applier")

			<-time.After(1 * time.Second)
			wg.Done()
			return
		}
	}
}
