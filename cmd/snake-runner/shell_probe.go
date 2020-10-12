package main

import (
	"context"
	"os/exec"
	"syscall"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/executor/shell"
	"github.com/reconquest/snake-runner/internal/platform"
	"github.com/reconquest/snake-runner/internal/sidecar"
)

//go:generate gonstructor --type=ShellProbe
type ShellProbe struct {
	shell *shell.Shell
}

func (probe *ShellProbe) Probe() (executor.Executor, error) {
	log.Debugf(nil, "checking shell executor prerequisites")

	err := sidecar.NewShellSidecarBuilder().
		Executor(probe.shell).
		Build().
		CheckPrerequisites(context.Background())
	if err != nil {
		return nil, karma.Format(
			err,
			"Prerequisites check failed while running snake-runner with SNAKE_EXEC_MODE=shell;"+
				" make sure that the specified binaries are installed."+
				"Read more: https://reconquest.link/EZJC1",
		)
	}

	if shell.PLATFORM == platform.WINDOWS {
		detected, err := probe.shell.DetectShell(context.Background(), nil)
		if err != nil {
			return nil, karma.Format(
				err,
				"unable to detect shell",
			)
		}

		if detected != shell.PREFERRED_SHELL {
			log.Warningf(
				nil,
				"no %s detected in the system; use of %s is recommended on Windows",
				shell.PREFERRED_SHELL,
				shell.PREFERRED_SHELL,
			)
		}

		container, err := probe.shell.Create(context.Background(), executor.CreateOptions{})
		if err != nil {
			return nil, karma.Format(
				err,
				"unable to create session for ssh-agent validation",
			)
		}

		log.Debugf(nil, "validating ssh-agent version")

		// Using the ability of ssh-agent to run a specified command.
		//
		// Correct version of ssh-agent will run command: in this case
		// "cmd.exe /c exit 100", so if no exit error is reported, then
		// the incorrect version of ssh-agent is used.
		openssh := false
		err = probe.shell.Exec(context.Background(), container, executor.ExecOptions{
			Cmd: []string{"ssh-agent", "cmd", "/c exit 100"},
		})
		if err != nil {
			if err, ok := err.(*exec.ExitError); ok {
				if status, ok := err.Sys().(syscall.WaitStatus); ok {
					// The git-bash's ssh-agent would exit with 100
					if status.ExitStatus() != 100 {
						openssh = true
					}
				}
			} else {
				return nil, karma.Format(
					err,
					"ssh-agent version validation failed",
				)
			}
		} else {
			openssh = true
		}

		if openssh {
			return nil, karma.Format(
				"You are probably using the OpenSSH's ssh-agent which is not supported.\n"+
					"Consider installing Git-BASH and adding the Git-BASH's bin folder as a part of the system $PATH.\n"+
					"Read more: https://reconquest.link/a56Pd",
				"The ssh-agent's version is not supported",
			)
		}
	}

	return probe.shell, nil
}
