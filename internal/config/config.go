package config

import (
	"errors"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/internal/mapslice"
	"gopkg.in/yaml.v3"
)

type Pipeline struct {
	Variables *mapslice.MapSlice `json:"variables" yaml:"variables"`
	Shell     string             `json:"shell"     yaml:"shell"`
	Image     string             `json:"image"     yaml:"image"`
	Stages    []string           `json:"stages"    yaml:"stages"`
	Jobs      map[string]Job     `json:"jobs"      yaml:"jobs"`
}

type Job struct {
	Variables *mapslice.MapSlice `json:"variables" yaml:"variables"`
	Stage     string             `json:"stage"     yaml:"stage"`
	Shell     string             `json:"shell"     yaml:"shell"`
	Image     string             `json:"image"     yaml:"image"`
	Commands  []string           `json:"commands"  yaml:"commands"`
}

func Unmarshal(data []byte) (Pipeline, error) {
	var config Pipeline

	raw := map[string]yaml.Node{}
	err := yaml.Unmarshal(data, &raw)
	if err != nil {
		return config, err
	}

	if node, ok := raw["image"]; ok {
		err = node.Decode(&config.Image)
		if err != nil {
			return config, karma.Format(
				err,
				"invalid yaml field: 'image'",
			)
		}

		delete(raw, "image")
	}

	if node, ok := raw["stages"]; !ok {
		return config, errors.New("missing stages field")
	} else {
		err = node.Decode(&config.Stages)
		if err != nil {
			return config, karma.Format(
				err,
				"invalid yaml field: 'stages'",
			)
		}

		delete(raw, "stages")
	}

	config.Jobs = map[string]Job{}
	for jobName, node := range raw {
		var job Job
		err := node.Decode(&job)
		if err != nil {
			return config, karma.Format(
				err,
				"invalid yaml job: '%s'", jobName,
			)
		}

		config.Jobs[jobName] = job
	}

	if node, ok := raw["variables"]; ok {
		config.Variables, err = mapslice.New(node)
		if err != nil {
			return config, karma.Format(
				err,
				"invalid yaml field: 'variables'",
			)
		}
	}

	return config, nil
}
