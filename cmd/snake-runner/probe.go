package main

import (
	"fmt"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/executor/docker"
	"github.com/reconquest/snake-runner/internal/executor/shell"
	"github.com/reconquest/snake-runner/internal/runner"
)

//go:generate gonstructor --type=ProbeFactory
type ProbeFactory struct {
	config *runner.Config
}

func (factory *ProbeFactory) Probe() (executor.Executor, error) {
	switch factory.config.Mode {
	case runner.RUNNER_MODE_DOCKER:
		log.Debugf(nil, "initializing docker provider")

		return NewDockerProbe(
			docker.NewDocker(
				factory.config.Docker.Network,
				factory.config.Docker.Volumes,
			),
		).Probe()

	case runner.RUNNER_MODE_SHELL:
		log.Debugf(nil, "initializing shell provider")

		return NewShellProbe(shell.NewShell()).Probe()

	default:
		return nil, fmt.Errorf(
			"unexpected runner mode: %s", factory.config.Mode,
		)
	}
}
