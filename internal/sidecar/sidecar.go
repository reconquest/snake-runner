package sidecar

import (
	"context"

	"github.com/reconquest/snake-runner/internal/spawner"
)

const (
	SSH_CONFIG_NO_VERIFICATION = `Host *
	StrictHostKeyChecking no
	UserKnownHostsFile /dev/null
`

	SUBDIR_GIT = `git`
	SUBDIR_SSH = `ssh`

	SSH_SOCKET_VAR      = `SSH_AUTH_SOCK`
	SSH_SOCKET_FILENAME = `ssh-agent.sock`
)

type Sidecar interface {
	Serve(context context.Context, cloneURL, commit string) error
	Destroy()

	GitDir() string
	SshSocketPath() string

	ReadFile(context context.Context, cwd, path string) (string, error)

	Capabilities() Capabilities
	// not all sidecars support these:
	ContainerVolumes() []spawner.Volume
}

type Capabilities interface {
	// Containers() bool
	Volumes() bool
}

type capabilities struct {
	// containers bool
	volumes bool
}

func (caps capabilities) Volumes() bool {
	return caps.volumes
}
