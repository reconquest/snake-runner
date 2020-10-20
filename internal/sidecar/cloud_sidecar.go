package sidecar

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/consts"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/sshkey"
)

//go:generate gonstructor -type CloudSidecar -constructorTypes builder

const (
	CLOUD_SIDECAR_IMAGE = "reconquest/snake-runner-sidecar"
)

var _ Sidecar = (*CloudSidecar)(nil)

// CloudSidecar produces the following volumes:
// * volume with git repository cloned from the bitbucket instance
// * volume with shared ssh-agent socket
type CloudSidecar struct {
	executor executor.Executor
	name     string
	// pipelinesDir is the directory on the host file system where all pipelines
	// and temporary stuff such as git repos and ssh sockets are stored
	pipelinesDir   string
	slug           string
	promptConsumer executor.PromptConsumer
	outputConsumer executor.OutputConsumer
	sshKey         sshkey.Key

	container executor.Container `gonstructor:"-"`

	hostSubDir   string `gonstructor:"-"`
	containerDir string `gonstructor:"-"`

	// directory with git repository cloned
	gitDir string `gonstructor:"-"`

	// directory with ssh-agent socket
	sshDir        string `gonstructor:"-"`
	sshSocket     string `gonstructor:"-"`
	sshKnownHosts string `gonstructor:"-"`

	volumes []executor.Volume

	sshAgent *sync.WaitGroup `gonstructor:"-"`
}

func (sidecar *CloudSidecar) ContainerVolumes() []executor.Volume {
	return []executor.Volume{
		executor.Volume(sidecar.hostSubDir + "/" + consts.SUBDIR_GIT + "/" + sidecar.slug + ":" + sidecar.gitDir),
		executor.Volume(sidecar.hostSubDir + "/" + consts.SUBDIR_SSH + ":" + sidecar.sshDir),
	}
}

func (sidecar *CloudSidecar) GitDir() string {
	return sidecar.gitDir
}

func (sidecar *CloudSidecar) SshSocketPath() string {
	return sidecar.sshSocket
}

func (sidecar *CloudSidecar) SshKnownHostsPath() string {
	return sidecar.sshKnownHosts
}

func (sidecar *CloudSidecar) Container() executor.Container {
	return sidecar.container
}

func (sidecar *CloudSidecar) create(ctx context.Context) error {
	err := sidecar.executor.Prepare(
		ctx,
		executor.PrepareOptions{
			Image:          CLOUD_SIDECAR_IMAGE,
			OutputConsumer: sidecar.outputConsumer,
			InfoConsumer:   executor.DiscardConsumer,
			Auths:          nil,
		},
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to prepare sidecar",
		)
	}

	sidecar.hostSubDir = filepath.Join(sidecar.pipelinesDir, sidecar.name)
	sidecar.containerDir = filepath.Join("/pipeline")

	sidecar.gitDir = filepath.Join(sidecar.containerDir, consts.SUBDIR_GIT, sidecar.slug)
	sidecar.sshDir = filepath.Join(sidecar.containerDir, consts.SUBDIR_SSH)

	volumes := []executor.Volume{
		executor.Volume(sidecar.hostSubDir + ":" + sidecar.containerDir + ":rw"),

		// host dir is bound because we are going to get rid of entire sidecar
		// subdir during container destroying
		executor.Volume(sidecar.pipelinesDir + ":/host:rw"),
	}

	volumes = append(volumes, sidecar.volumes...)

	sidecar.container, err = sidecar.executor.Create(
		ctx,
		executor.CreateOptions{
			Name:    "snake-runner-sidecar-" + sidecar.name,
			Image:   CLOUD_SIDECAR_IMAGE,
			Volumes: volumes,
		},
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to create sidecar container",
		)
	}

	return nil
}

func (sidecar *CloudSidecar) Serve(
	ctx context.Context,
	opts ServeOptions,
) error {
	// creating container
	err := sidecar.create(ctx)
	if err != nil {
		return err
	}

	// preparing _directories_ such as 'git' and 'ssh'
	err = sidecar.executor.Exec(ctx, sidecar.container, executor.ExecOptions{
		Cmd:            []string{"mkdir", "-p", sidecar.gitDir, sidecar.sshDir},
		AttachStdout:   true,
		AttachStderr:   true,
		OutputConsumer: sidecar.getLogger("mkdir"),
	})
	if err != nil {
		return karma.Format(
			err,
			"unable to create directories: %s %s",
			sidecar.gitDir,
			sidecar.sshDir,
		)
	}

	// starting ssh-agent
	sidecar.sshAgent, sidecar.sshSocket, err = startSshAgent(
		ctx,
		sidecar.executor,
		sidecar.container,
		sidecar.getLogger("ssh-agent"),
		sidecar.sshDir,
	)
	if err != nil {
		return karma.Format(err, "unable to start ssh-agent")
	}

	sidecar.sshKnownHosts = filepath.Join(sidecar.sshDir, "known_hosts")
	knownHostsContent := joinKnownHosts(opts.KnownHosts)

	env := []string{
		consts.SSH_AUTH_SOCK_VAR + "=" + sidecar.sshSocket,
		"__SNAKE_PRIVATE_KEY=" + string(sidecar.sshKey.Private),
		"__SNAKE_SSH_KNOWN_HOSTS=" + knownHostsContent,
	}

	log.Tracef(
		karma.Describe("data", knownHostsContent),
		"[sidecar] known_hosts",
	)

	// adding ssh key to the ssh-agent and tuning up git a bit
	basic := []string{
		`mkdir -vp ~/.ssh`,
		`ssh-add -v - <<< "$__SNAKE_PRIVATE_KEY"`,
		`cat > ` + sidecar.sshKnownHosts + ` <<< "$__SNAKE_SSH_KNOWN_HOSTS"`,
		`cp -v ` + sidecar.sshKnownHosts + ` ~/.ssh/known_hosts`,
		`git config --global advice.detachedHead false`,
	}

	cmd := []string{"bash", "-c", strings.Join(basic, " && ")}

	err = sidecar.executor.Exec(ctx, sidecar.container, executor.ExecOptions{
		Cmd:            cmd,
		Env:            env,
		AttachStdout:   true,
		AttachStderr:   true,
		OutputConsumer: sidecar.getLogger("prepare"),
	})
	if err != nil {
		return karma.Describe("cmd", cmd).Format(
			err,
			"unable to prepare sidecar container",
		)
	}

	// cloning git repository and switching commit
	commands := [][]string{
		{"git", "-C", sidecar.gitDir, "clone", "--recursive", opts.CloneURL, "."},
		{"git", "-C", sidecar.gitDir, "checkout", opts.Commit},
	}

	for _, cmd := range commands {
		sidecar.promptConsumer(cmd)

		err = sidecar.executor.Exec(ctx, sidecar.container, executor.ExecOptions{
			Env: []string{
				consts.SSH_AUTH_SOCK_VAR + "=" + sidecar.sshSocket,
				// NOTE: the private key is not passed anymore but it's already
				// in ssh-agent's memory
			},
			Cmd:            cmd,
			AttachStdout:   true,
			AttachStderr:   true,
			OutputConsumer: sidecar.outputConsumer,
		})
		if err != nil {
			return karma.
				Describe("cmd", cmd).
				Format(err, "unable to setup repository")
		}
	}

	return nil
}

func (sidecar *CloudSidecar) getLogger(tag string) func(string) {
	return func(text string) {
		log.Debugf(
			nil,
			"[sidecar] %s {%s}: %s",
			sidecar.container.String(), tag, strings.TrimRight(text, "\n"),
		)
	}
}

func (sidecar *CloudSidecar) Destroy() {
	if sidecar.container == nil {
		return
	}
	// we use Background context here because local ctx can be destroyed
	// already

	if sidecar.name != "" {
		cmd := []string{"rm", "-rf", filepath.Join("/host", sidecar.name)}

		log.Debugf(
			nil,
			"cleaning up sidecar %s container: %v",
			sidecar.container.String(), cmd,
		)

		err := sidecar.executor.Exec(
			context.Background(),
			sidecar.container,
			executor.ExecOptions{
				Cmd:            cmd,
				AttachStderr:   true,
				AttachStdout:   true,
				OutputConsumer: sidecar.getLogger("rm"),
			},
		)
		if err != nil {
			log.Errorf(
				err,
				"unable to cleanup sidecar directory: %s %s",
				sidecar.GitDir(),
				sidecar.hostSubDir,
			)
		}
	}

	log.Debugf(
		nil,
		"destroying sidecar %s container",
		sidecar.container.String(),
	)

	err := sidecar.executor.Destroy(
		context.Background(),
		sidecar.container,
	)
	if err != nil {
		log.Errorf(
			err,
			"unable to destroy sidecar container %s",
			sidecar.container.String(),
		)

		return
	}

	// wait for ssh-agent to die in order to avoid go routine leakage
	sidecar.sshAgent.Wait()
}

func (sidecar *CloudSidecar) ReadFile(ctx context.Context, cwd, path string) (string, error) {
	data := ""
	callback := func(line string) {
		if data == "" {
			data = line
		} else {
			data = data + "\n" + line
		}
	}

	err := sidecar.executor.Exec(
		ctx,
		sidecar.Container(),
		executor.ExecOptions{
			AttachStdout:   true,
			AttachStderr:   true,
			Cmd:            []string{"cat", path},
			WorkingDir:     cwd,
			OutputConsumer: callback,
		},
	)
	if err != nil {
		return "", err
	}

	return data, nil
}
