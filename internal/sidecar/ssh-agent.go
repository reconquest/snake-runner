package sidecar

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/consts"
	"github.com/reconquest/snake-runner/internal/executor"
)

func startSshAgent(
	ctx context.Context,
	execer executor.Executor,
	container executor.Container,
	logger func(string),
	sshDir string,
) (*sync.WaitGroup, string, error) {
	started := make(chan struct{}, 1)
	failed := make(chan error, 1)

	callback := func(text string) {
		logger(text)

		if strings.Contains(text, consts.SSH_AUTH_SOCK_VAR+"=") {
			started <- struct{}{}
		}
	}

	sshSocket := filepath.Join(sshDir, consts.SSH_SOCKET_FILENAME)

	sshAgent := &sync.WaitGroup{}
	sshAgent.Add(1)
	go func() {
		defer audit.Go("sidecar", "ssh-agent")()

		defer sshAgent.Done()

		err := execer.Exec(ctx, container, executor.ExecOptions{
			Cmd: []string{
				"ssh-agent",
				"-d", "-a", sshSocket,
			},

			AttachStdout:   true,
			AttachStderr:   true,
			OutputConsumer: callback,
		},
		)
		if err != nil {
			failed <- karma.Format(
				err,
				"unable to run ssh-agent in sidecar container",
			)
		}
	}()

	select {
	case <-ctx.Done():
		return sshAgent, "", context.Canceled

	case err := <-failed:
		return sshAgent, "", err

	case <-started:
		return sshAgent, sshSocket, nil
	}
}
