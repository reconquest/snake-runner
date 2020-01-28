package main

import (
	"errors"

	"github.com/reconquest/karma-go"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Shell  string               `json:"shell"`
	Image  string               `json:"image"`
	Stages []string             `json:"stages"`
	Jobs   map[string]ConfigJob `json:"jobs"`
}

type ConfigJob struct {
	Stage    string   `yaml:"stage"`
	Shell    string   `yaml:"shell"`
	Image    string   `yaml:"image"`
	Commands []string `yaml:"commands"`
}

func unmarshalConfig(data []byte) (*Config, error) {
	var config Config

	raw := map[string]yaml.Node{}
	err := yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, err
	}

	if node, ok := raw["image"]; !ok {
		return nil, errors.New("missing image field")
	} else {
		err = node.Decode(&config.Image)
		if err != nil {
			return nil, karma.Format(
				err,
				"invalid yaml field: 'image'",
			)
		}

		delete(raw, "image")
	}

	if node, ok := raw["stages"]; !ok {
		return nil, errors.New("missing stages field")
	} else {
		err = node.Decode(&config.Stages)
		if err != nil {
			return nil, karma.Format(
				err,
				"invalid yaml field: 'stages'",
			)
		}

		delete(raw, "stages")
	}

	config.Jobs = map[string]ConfigJob{}
	for jobName, node := range raw {
		var job ConfigJob
		err := node.Decode(&job)
		if err != nil {
			return nil, karma.Format(
				err,
				"invalid yaml job: '%s'", jobName,
			)
		}

		config.Jobs[jobName] = job
	}

	return &config, nil
}
