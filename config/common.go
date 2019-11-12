package config

import (
	"io/ioutil"

	"github.com/aschmidt75/ipvsmesh/model"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func readInput(filename string) ([]byte, error) {
	var b []byte
	var err error
	b, err = ioutil.ReadFile(filename)
	if err != nil {
		log.Errorf("Error reading from input file %s", filename)
	}

	return b, err
}

func ReadModelFromInput(filename string) (*model.IPVSMeshConfig, error) {
	c := &model.IPVSMeshConfig{}

	b, err := readInput(filename)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(b, c)
	if err != nil {
		log.Errorf("Error parsing yaml")
	}

	return c, err
}
