package sidecar

import (
	"context"

	"github.com/reconquest/snake-runner/internal/env"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/responses"
)

type Sidecar interface {
	Serve(context context.Context, options ServeOptions) error
	Destroy()

	GitDir() string
	SshSocketPath() string
	SshKnownHostsPath() string

	ContainerVolumes() []executor.Volume

	ReadFile(context context.Context, cwd, path string) (string, error)
}

type ServeOptions struct {
	Env        *env.Env
	KnownHosts []responses.KnownHost
	CloneURL   string
	Commit     string
}
