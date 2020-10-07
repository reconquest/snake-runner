package main

import (
	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/internal/executor/docker"
	"github.com/reconquest/snake-runner/internal/platform"
)

//go:generate gonstructor --type=DockerProbe
type DockerProbe struct {
	docker *docker.Docker
}

func (probe *DockerProbe) Probe() (*docker.Docker, error) {
	err := probe.docker.Connect()
	if err != nil {
		if platform.CURRENT == platform.WINDOWS {
			// TODO: confirm the link
			return nil, karma.Format(
				err,
				"Unable to connect to Docker Engine. Is it installed and running?\n"+
					"Consider specifying the SNAKE_EXEC_MODE=shell environment variable "+
					"or putting exec_mode:shell into your configuration file to use the shell executor.\n"+
					"Read more: https://docs-next.snake-ci.com/docs/getting-started/setup-runner/windows/",
			)
		}

		return nil, karma.Format(
			err,
			"Unable to connect to Docker Engine. Is it installed and running?",
		)
	}

	return probe.docker, nil
}
