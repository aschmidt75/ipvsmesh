package proxyfromfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aschmidt75/ipvsmesh/model"
	"github.com/radovskyb/watcher"
	log "github.com/sirupsen/logrus"
)

// Spec is the spec subpart of a service for the docker front proxy plugin
type Spec struct {
	File          string `yaml:"file"`
	Type          string `yaml:"type"`
	DefaultWeight int    `yaml:"defaultWeight"`

	mu          sync.Mutex
	lastModTime time.Time
}

// Initialize the plugin
func (s *Spec) Initialize(globals *model.Globals) error {
	return nil
}

// Name returns the plugin name
func (s *Spec) Name() string {
	return "proxyFromFile"
}

// HasDownwardInterface is true, plugin checks for file updates
func (s *Spec) HasDownwardInterface() bool {
	return true
}

// GetDownwardData ..
func (s *Spec) GetDownwardData() ([]model.DownwardBackendServer, error) {
	res := []model.DownwardBackendServer{}

	var b []byte
	var err error
	b, err = ioutil.ReadFile(s.File)
	if err != nil {
		return res, err
	}

	if s.Type == "text" {
		return s.getDownwardDataText(b)
	}
	if s.Type == "json" {
		return s.getDownwardDataJSON(b)
	}

	return res, errors.New("invalid type")
}

func (s *Spec) getDownwardDataJSON(b []byte) ([]model.DownwardBackendServer, error) {
	res := []model.DownwardBackendServer{}

	var f interface{}

	err := json.Unmarshal(b, &f)
	if err != nil {
		return res, err
	}

	//	log.WithField("f", f).Trace("Parsed.")
	switch f.(type) {
	case []interface{}:
		l := f.([]interface{})
		for _, lx := range l {
			switch lx.(type) {
			case map[string]interface{}:
				m := lx.(map[string]interface{})
				//				log.WithField("m", m).Trace("Parsing")

				ip0, ex1 := m["ip"]
				weight0, ex2 := m["weight"]

				added := false
				if ip, ok := ip0.(string); ex1 && ok {
					if weight, ok2 := weight0.(float64); ex2 && ok2 {
						// todo: validate data
						res = append(res, model.DownwardBackendServer{
							Address: ip,
							Weight:  int(weight),
						})
						added = true
					} else {
						log.WithField("m", m).Warn("weight not valid")
					}
				} else {
					log.WithField("m", m).Warn("ip not valid")
				}
				if !added {
					log.WithField("m", m).Warn("invalid data")
				}
			default:
				log.WithField("lx", lx).Trace("skipping, not a map.")
			}
		}
	default:
		log.WithField("f", f).Trace("skipping, not a list.")
	}
	return res, nil
}

func (s *Spec) getDownwardDataText(b []byte) ([]model.DownwardBackendServer, error) {
	res := []model.DownwardBackendServer{}

	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if len(strings.Trim(line, " \t\n\r")) == 0 {
			continue
		}

		// expect (IP|Host)[:PORT] [WEIGHT]
		h, p, w, err := splitHostPortWeight(line)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
				"l":   line,
			}).Error("proxy-from-file: Skipping malformed line")
			continue
		}

		var a string
		if p == 0 {
			a = h
		} else {
			a = fmt.Sprintf("%s:%d", h, p)
		}

		if w == 0 {
			w = s.DefaultWeight
		}

		dbs := model.DownwardBackendServer{
			Address: a,
			Weight:  w,
		}
		log.WithFields(log.Fields{
			"l":    line,
			"data": dbs,
		}).Trace("proxy-from-file: processed line")
		res = append(res, dbs)
	}

	return res, nil
}

func splitHostPortWeight(in string) (host string, port int, weight int, err error) {
	i := strings.LastIndex(in, " ")
	if i == -1 {
		h, p, err := splitHostPort(in)
		return h, p, 0, err
	}

	a := strings.Split(in, " ")
	if len(a) != 2 {
		return "", 0, 0, errors.New("parse error in " + in)
	}
	w, err := strconv.ParseInt(a[1], 10, 32)
	if err != nil {
		return "", 0, 0, err
	}

	h, p, err := splitHostPort(a[0])
	return h, p, int(w), err
}

func splitHostPort(in string) (host string, port int, err error) {
	// 1. todo: check for ipv6 addr e.g. [fe80]

	i := strings.LastIndex(in, ":")
	if i == -1 {
		// no ":", no port there
		return in, 0, nil
	}

	a := strings.Split(in, ":")
	if len(a) != 2 {
		return "", 0, errors.New("parse error in " + in)
	}
	p, err := strconv.ParseInt(a[1], 10, 32)
	if err != nil {
		return "", 0, err
	}

	return a[0], int(p), nil
}

// HasUpwardInterface is false, does not expose something
func (s *Spec) HasUpwardInterface() bool {
	return false
}

// RunNotificationLoop ...
func (s *Spec) RunNotificationLoop(notChan chan struct{}, quitChan chan struct{}) error {
	log.WithField("Name", s.Name()).Debug("proxy-from-file: Starting notification loop")

	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write, watcher.Create, watcher.Remove)

	if err := w.Add(s.File); err != nil {
		log.WithField("err", err).Error("proxy-from-file: Unable to set up watcher")
	}

	go func() {
		w.Wait()
		log.Debug("proxy-from-file: Initial config file read trigger")
		w.TriggerEvent(watcher.Create, nil)
	}()

	go func() {
		if err := w.Start(time.Millisecond * 100); err != nil {
			log.WithField("err", err).Error("proxy-from-file: Unable to start watcher")
		}

	}()

	for {
		select {
		case event := <-w.Event:
			log.WithField("e", event).Debug("proxy-from-file: config file(s) changed")
			info, err := os.Stat(s.File)
			if err == nil {
				mt := info.ModTime()
				if mt.After(s.lastModTime) {
					notChan <- struct{}{}
					s.lastModTime = mt
				}
			}
		case err := <-w.Error:
			log.WithField("err", err).Error("proxy-from-file: Watcher error")
		case <-w.Closed:
			log.Info("proxy-from-file: Stopping Configuratiom Watcher")
			return nil
		case <-quitChan:
			log.WithField("Name", s.Name()).Debug("proxy-from-file: Stopped notification loop")
			return nil
		}
	}
}

func (s *Spec) PushUpwardData(data model.UpwardData) error {
	return nil
}
