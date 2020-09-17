package sidecar

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/consts"
	"github.com/reconquest/snake-runner/internal/spawner"
	"github.com/reconquest/snake-runner/internal/sshkey"
)

//go:generate gonstructor -type ShellSidecar -constructorTypes builder

type ShellSidecar struct {
	spawner        spawner.Spawner
	name           string
	slug           string
	promptConsumer spawner.PromptConsumer
	outputConsumer spawner.OutputConsumer
	pipelinesDir   string

	baseDir string `gonstructor:"-"`
	tempDir string `gonstructor:"-"`
	gitDir  string `gonstructor:"-"`

	sshKey        sshkey.Key
	sshSocket     string          `gonstructor:"-"`
	sshAgent      *sync.WaitGroup `gonstructor:"-"`
	sshKnownHosts string          `gonstructor:"-"`

	container spawner.Container `gonstructor:"-"`
}

func (sidecar *ShellSidecar) getTempDir() (string, error) {
	return ioutil.TempDir("", "snake-runner.*")
}

func (sidecar *ShellSidecar) Serve(
	ctx context.Context,
	opts ServeOptions,
) error {
	baseDir := filepath.Join(sidecar.pipelinesDir, sidecar.name)

	sidecar.baseDir = baseDir
	sidecar.gitDir = filepath.Join(baseDir, consts.SUBDIR_GIT, sidecar.slug)

	var err error
	sidecar.tempDir, err = sidecar.getTempDir()
	if err != nil {
		return karma.Format(err, "unable to obtain temporary directory")
	}

	err = os.MkdirAll(sidecar.gitDir, 0o755)
	if err != nil {
		return karma.Format(
			err,
			"unable to create directory for git: %s", sidecar.gitDir,
		)
	}

	container, err := sidecar.spawner.Create(
		ctx,
		spawner.CreateOptions{
			Name: "snake-runner-sidecar-" + sidecar.name,
		},
	)
	if err != nil {
		return err
	}

	sidecar.container = container

	// starting ssh-agent
	sidecar.sshAgent, sidecar.sshSocket, err = startSshAgent(
		ctx,
		sidecar.spawner,
		sidecar.container,
		sidecar.getLogger("ssh-agent"),
		sidecar.tempDir,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to start ssh-agent",
		)
	}

	sidecar.sshKnownHosts = filepath.Join(sidecar.tempDir, "known_hosts")

	err = ioutil.WriteFile(
		sidecar.sshKnownHosts,
		[]byte(joinKnownHosts(opts.KnownHosts)),
		0o644,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to save known hosts file",
		)
	}

	env := append(os.Environ(), []string{
		consts.SSH_AUTH_SOCK_VAR + "=" + sidecar.sshSocket,
		consts.GIT_SSH_COMMAND_VAR + "=" + "ssh -o" + consts.SSH_OPTION_GLOBAL_HOSTS_FILE + "=" + sidecar.sshKnownHosts,

		// NOTE: the private key is not passed anymore but it's already
		// in ssh-agent's memory
	}...)

	steps := []struct {
		prompt bool
		cmd    []string
		stdin  io.Reader
	}{
		{
			prompt: false,
			cmd:    []string{"ssh-add", "-v", "-"},
			stdin:  bytes.NewBufferString(sidecar.sshKey.Private),
		},
		{
			prompt: true,
			cmd:    []string{"git", "clone", "--recursive", opts.CloneURL, sidecar.gitDir},
		},
		{
			prompt: false,
			cmd:    []string{"git", "-C", sidecar.gitDir, "config", "advice.detachedHead", "false"},
		},
		{
			prompt: true,
			cmd:    []string{"git", "-C", sidecar.gitDir, "checkout", opts.Commit},
		},
	}

	for _, step := range steps {
		log.Tracef(nil, "[sidecar] start %q", step.cmd)

		if step.prompt {
			sidecar.promptConsumer(step.cmd)
		}

		err = sidecar.spawner.Exec(ctx, sidecar.container, spawner.ExecOptions{
			Env:            env,
			Cmd:            step.cmd,
			Stdin:          step.stdin,
			AttachStdout:   true,
			AttachStderr:   true,
			OutputConsumer: sidecar.outputConsumer,
		})

		log.Tracef(nil, "[sidecar] start %q", step.cmd)

		if err != nil {
			return karma.
				Describe("cmd", fmt.Sprintf("%q", step.cmd)).
				Format(err, "unable to setup repository")
		}
	}

	return nil
}

func (sidecar *ShellSidecar) Destroy() {
	if sidecar.container != nil {
		err := sidecar.spawner.Destroy(context.Background(), sidecar.container)
		if err != nil {
			log.Errorf(err, "unable to destroy resources")
		}
	}

	if sidecar.sshAgent != nil {
		log.Debug("waiting for ssh-agent to stop")
		sidecar.sshAgent.Wait()
	}

	log.Errorf(nil, "removing directory: "+sidecar.baseDir)
	//err := os.RemoveAll(sidecar.baseDir)
	//if err != nil {
	//    log.Errorf(err, "unable to remove git/ssh directories")
	//}

	return
}

func (sidecar *ShellSidecar) GitDir() string {
	return sidecar.gitDir
}

func (sidecar *ShellSidecar) SshSocketPath() string {
	return sidecar.sshSocket
}

func (sidecar *ShellSidecar) SshKnownHostsPath() string {
	return sidecar.sshKnownHosts
}

func (sidecar *ShellSidecar) ReadFile(
	context context.Context,
	cwd, path string,
) (string, error) {
	contents, err := ioutil.ReadFile(filepath.Join(cwd, path))
	if err != nil {
		return "", err
	}

	return string(contents), nil
}

func (sidecar *ShellSidecar) ContainerVolumes() []spawner.Volume {
	return nil
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
