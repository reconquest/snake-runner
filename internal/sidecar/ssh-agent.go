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
	"github.com/reconquest/snake-runner/internal/utils"
)

func startSshAgent(
	ctx context.Context,
	execer executor.Executor,
	container executor.Container,
	logger func(string),
	sshDir string,
) (*sync.WaitGroup, string, error) {
	chError := make(chan error, 1)
	chStarted := make(chan struct{}, 1)

	callback := func(text string) {
		logger(text)

		if strings.Contains(text, consts.SSH_AUTH_SOCK_VAR+"=") {
			chStarted <- struct{}{}
		}
	}

	sshSocket := filepath.Join(sshDir, consts.SSH_SOCKET_FILENAME)

	sshAgent := &sync.WaitGroup{}
	sshAgent.Add(1)

	go func() {
		defer audit.Go("sidecar", "ssh-agent")()

		defer sshAgent.Done()

		logger("process is starting")

		cmd := []string{
			"ssh-agent", "-d", "-a", sshSocket,
		}

		err := execer.Exec(ctx, container, executor.ExecOptions{
			Cmd: cmd,

			AttachStdout:   true,
			AttachStderr:   true,
			OutputConsumer: callback,
		})

		logger("process has stopped")

		if err != nil {
			if utils.IsCanceled(err) {
				return
			}

			chError <- karma.Format(
				err,
				"unable to start ssh-agent",
			)
			return
		}

		chError <- karma.Describe("cmd", cmd).Format(
			"The ssh-agent process has chStarted and did not output the path to socket.\n"+
				"You are probably using OpenSSH's ssh-agent which is not supported.\n"+
				"Consider installing Git-BASH and adding Git-BASH's bin as a part of the system $PATH.\n"+
				"Read more: https://snake-ci.com/docs/throubleshoot/windows-ssh-agent/",
			"unable to start ssh-agent",
		)
	}()

	select {
	case <-ctx.Done():
		return sshAgent, "", context.Canceled

	case <-chStarted:
		return sshAgent, sshSocket, nil

	case err := <-chError:
		return sshAgent, "", err
	}
}
