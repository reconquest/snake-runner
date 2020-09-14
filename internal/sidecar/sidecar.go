package sidecar

import (
	"context"

	"github.com/reconquest/snake-runner/internal/env"
	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/spawner"
)

const (
	SUBDIR_GIT          = `git`
	SUBDIR_SSH          = `ssh`
	SSH_SOCKET_VAR      = `SSH_AUTH_SOCK`
	SSH_SOCKET_FILENAME = `ssh-agent.sock`

	SSH_CONFIG_NO_STRICT_HOST_KEY_CHECKING = `Host *
	StrictHostKeyChecking no
	UserKnownHostsFile /dev/null
`
)

type Sidecar interface {
	Serve(context context.Context, options ServeOptions) error
	Destroy()

	GitDir() string
	SshSocketPath() string
	ContainerVolumes() []spawner.Volume

	ReadFile(context context.Context, cwd, path string) (string, error)
}

type ServeOptions struct {
	Env        *env.Env
	KnownHosts []responses.KnownHost
	CloneURL   string
	Commit     string
}
