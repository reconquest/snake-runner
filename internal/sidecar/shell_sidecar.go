package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/spawner"
)

var capabilitiesShell = capabilities{}

type ShellSidecar struct {
	spawner        spawner.Spawner
	name           string
	slug           string
	promptConsumer spawner.PromptConsumer
	outputConsumer spawner.OutputConsumer
	pipelinesDir   string
	gitDir         string `gonstructor:"-"`
	sshDir         string `gonstructor:"-"`
	sshAgent       sync.WaitGroup

	container spawner.Container `gonstructor:"-"`
}

func (sidecar *ShellSidecar) Capabilities() Capabilities {
	return capabilitiesShell
}

func (sidecar *ShellSidecar) Serve(
	ctx context.Context,
	cloneURL,
	commit string,
) error {
	baseDir := filepath.Join(sidecar.pipelinesDir, sidecar.name)

	sidecar.gitDir = filepath.Join(baseDir, SUBDIR_GIT, sidecar.slug)
	sidecar.sshDir = filepath.Join(baseDir, SUBDIR_SSH)

	err := os.MkdirAll(sidecar.gitDir, 0o755)
	if err != nil {
		return karma.Format(
			err,
			"unable to create directory for git: %s", sidecar.gitDir,
		)
	}

	err = os.MkdirAll(sidecar.sshDir, 0o755)
	if err != nil {
		return karma.Format(
			err,
			"unable to create directory for ssh agent: %s", sidecar.gitDir,
		)
	}

	container, err := sidecar.spawner.Create(
		ctx,
		spawner.Name("snake-runner-sidecar-"+sidecar.name),
		spawner.Image(""),
		[]spawner.Volume{},
	)
	if err != nil {
		return err
	}

	sidecar.container = container

	config := spawner.ExecConfig{
		Env:          os.Environ(),
		AttachStdout: true,
		AttachStderr: true,
	}

	err = sidecar.spawner.Exec(
		ctx,
		container,
		config,
		sidecar.outputConsumer,
	)
	if err != nil {
		return err
	}

	return nil
}

func (sidecar *ShellSidecar) startSshAgent(ctx context.Context) (string, error) {
	started := make(chan struct{}, 1)
	failed := make(chan error, 1)

	logger := sidecar.getLogger("ssh-agent")
	callback := func(text string) {
		logger(text)

		if strings.Contains(text, SSH_SOCKET_VAR+"=") {
			started <- struct{}{}
		}
	}

	sshSocket := filepath.Join(sidecar.sshDir, SSH_SOCKET_FILENAME)

	sidecar.sshAgent.Add(1)
	go func() {
		defer audit.Go("sidecar", "ssh-agent")()

		defer sidecar.sshAgent.Done()

		err := sidecar.spawner.Exec(ctx, sidecar.container, spawner.ExecConfig{
			Cmd: []string{
				"ssh-agent",
				"-d", "-a", sshSocket,
			},

			AttachStdout: true,
			AttachStderr: true,
		}, callback)
		if err != nil {
			failed <- karma.Format(
				err,
				"unable to run ssh-agent in sidecar container",
			)
		}
	}()

	select {
	case <-ctx.Done():
		return "", context.Canceled

	case err := <-failed:
		return "", err

	case <-started:
		return sshSocket, nil
	}
}

func (sidecar *ShellSidecar) Destroy() {
	if sidecar.container != nil {
		err := sidecar.spawner.Destroy(context.Background(), sidecar.container)
		if err != nil {
			log.Errorf(err, "unable to destroy resources")
		}
	}

	return
}

func (sidecar *ShellSidecar) GitDir() string {
	return ""
}

func (sidecar *ShellSidecar) SshSocketPath() string {
	return ""
}

func (sidecar *ShellSidecar) ReadFile(
	context context.Context,
	cwd, path string,
) (string, error) {
	return "", nil
}

func (sidecar *ShellSidecar) ContainerVolumes() []string {
	panic(
		"BUG: Shell sidecar is not capable of container volumes " +
			"and this method should not be invoked",
	)
}

func (sidecar *ShellSidecar) getLogger(tag string) func(string) {
	return func(text string) {
		log.Debugf(
			nil,
			"[sidecar] %s {%s}: %s",
			sidecar.name, tag, strings.TrimRight(text, "\n"),
		)
	}
}
