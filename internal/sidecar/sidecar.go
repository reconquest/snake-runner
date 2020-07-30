package sidecar

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/sshkey"
)

//go:generate gonstructor -type Sidecar -constructorTypes builder

const (
	SidecarImage = "reconquest/snake-runner-sidecar"
)

const SSHConfigWithoutVerification = `Host *
	StrictHostKeyChecking no
	UserKnownHostsFile /dev/null
`

const (
	GitSubDir = `git`
	SshSubDir = `ssh`

	SshSockVar  = `SSH_AUTH_SOCK`
	SshSockFile = `ssh-agent.sock`
)

// Sidecar produces the following volumes:
// * volume with git repository cloned from the bitbucket instance
// * volume with shared ssh-agent socket
type Sidecar struct {
	cloud cloud.Cloud
	name  string
	// pipelinesDir is the directory on the host file system where all pipelines
	// and temporary stuff such as git repos and ssh sockets are stored
	pipelinesDir   string
	slug           string
	promptConsumer cloud.PromptConsumer
	outputConsumer cloud.OutputConsumer
	sshKey         sshkey.Key
	pullConfigs    []cloud.PullConfig

	container cloud.Container `gonstructor:"-"`

	hostSubDir   string `gonstructor:"-"`
	containerDir string `gonstructor:"-"`

	// directory with git repository cloned
	gitDir string `gonstructor:"-"`

	// directory with ssh-agent socket
	sshDir string `gonstructor:"-"`

	sshAgent sync.WaitGroup
}

func (sidecar *Sidecar) GetContainerVolumes() []string {
	return []string{
		sidecar.hostSubDir + "/" + GitSubDir + ":" + sidecar.gitDir,
		sidecar.hostSubDir + "/" + SshSubDir + ":" + sidecar.sshDir,
	}
}

func (sidecar *Sidecar) GetGitDir() string {
	return sidecar.gitDir
}

func (sidecar *Sidecar) GetSshDir() string {
	return sidecar.sshDir
}

func (sidecar *Sidecar) GetContainer() cloud.Container {
	return sidecar.container
}

func (sidecar *Sidecar) create(ctx context.Context) error {
	img, err := sidecar.cloud.GetImageWithTag(ctx, SidecarImage)
	if err != nil {
		return err
	}

	if img == nil {
		err := sidecar.cloud.PullImage(
			ctx,
			SidecarImage,
			sidecar.outputConsumer,
			sidecar.pullConfigs,
		)
		if err != nil {
			return karma.Format(
				err,
				"unable to pull sidecar image: %s", SidecarImage,
			)
		}
	}

	sidecar.hostSubDir = filepath.Join(sidecar.pipelinesDir, sidecar.name)
	sidecar.containerDir = filepath.Join("/pipelines", sidecar.slug)

	sidecar.gitDir = filepath.Join(sidecar.containerDir, GitSubDir)
	sidecar.sshDir = filepath.Join(sidecar.containerDir, SshSubDir)

	volumes := []string{
		sidecar.hostSubDir + ":" + sidecar.containerDir + ":rw",

		// host dir is bound because we are going to get rid of entire sidecar
		// subdir during container destroying
		sidecar.pipelinesDir + ":/host:rw",
	}

	sidecar.container, err = sidecar.cloud.CreateContainer(
		ctx,
		SidecarImage,
		"snake-runner-sidecar-"+sidecar.name,
		volumes,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to create sidecar container",
		)
	}

	return nil
}

func (sidecar *Sidecar) Serve(
	ctx context.Context,
	cloneURL string,
	commitish string,
) error {
	// creating container
	err := sidecar.create(ctx)
	if err != nil {
		return err
	}

	// preparing _directories_ such as 'git' and 'ssh'
	err = sidecar.cloud.Exec(ctx, sidecar.container, cloud.ExecConfig{
		Cmd:          []string{"mkdir", "-p", sidecar.gitDir, sidecar.sshDir},
		AttachStdout: true,
		AttachStderr: true,
	}, sidecar.getLogger("mkdir"))
	if err != nil {
		return karma.Format(
			err,
			"unable to create directories: %s %s",
			sidecar.gitDir,
			sidecar.sshDir,
		)
	}

	// starting ssh-agent
	sshSock, err := sidecar.startSshAgent(ctx)
	if err != nil {
		return karma.Format(
			err,
			"unable to start ssh-agent",
		)
	}

	env := []string{
		SshSockVar + "=" + sshSock,
		"__SNAKE_PRIVATE_KEY=" + string(sidecar.sshKey.Private),
		"__SNAKE_SSH_CONFIG=" + SSHConfigWithoutVerification,
	}

	// adding ssh key to the ssh-agent and tuning up git a bit
	basic := []string{
		`mkdir ~/.ssh`,
		`ssh-add -v - <<< "$__SNAKE_PRIVATE_KEY"`,
		`cat > ~/.ssh/config <<< "$__SNAKE_SSH_CONFIG"`,
		`git config --global advice.detachedHead false`,
	}

	cmd := []string{"bash", "-c", strings.Join(basic, " && ")}

	err = sidecar.cloud.Exec(ctx, sidecar.container, cloud.ExecConfig{
		Cmd:          cmd,
		Env:          env,
		AttachStdout: true,
		AttachStderr: true,
	}, sidecar.getLogger("prep"))
	if err != nil {
		return karma.Describe("cmd", cmd).Format(
			err,
			"unable to prepare sidecar container",
		)
	}

	// cloning git repository and switching commit
	commands := [][]string{
		{`git`, `clone`, "--recursive", cloneURL, sidecar.gitDir},
		{`git`, `-C`, sidecar.gitDir, `checkout`, commitish},
	}

	for _, cmd := range commands {
		sidecar.promptConsumer(cmd)

		err = sidecar.cloud.Exec(ctx, sidecar.container, cloud.ExecConfig{
			Env: []string{
				SshSockVar + "=" + sshSock,
				// NOTE: the private key is not passed anymore but it's already
				// in ssh-agent's memory
			},
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		}, sidecar.outputConsumer)
		if err != nil {
			return karma.
				Describe("cmd", cmd).
				Format(err, "unable to setup repository")
		}
	}

	return nil
}

func (sidecar *Sidecar) startSshAgent(ctx context.Context) (string, error) {
	started := make(chan struct{}, 1)
	failed := make(chan error, 1)

	logger := sidecar.getLogger("ssh-agent")
	callback := func(text string) {
		logger(text)

		if strings.Contains(text, SshSockVar+"=") {
			started <- struct{}{}
		}
	}

	sshSocket := filepath.Join(sidecar.sshDir, SshSockFile)

	sidecar.sshAgent.Add(1)
	go func() {
		defer audit.Go("sidecar", "ssh-agent")()

		defer sidecar.sshAgent.Done()

		err := sidecar.cloud.Exec(ctx, sidecar.container, cloud.ExecConfig{
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

func (sidecar *Sidecar) getLogger(tag string) func(string) {
	return func(text string) {
		log.Debugf(
			nil,
			"[sidecar] %s {%s}: %s",
			sidecar.container.String(), tag, strings.TrimRight(text, "\n"),
		)
	}
}

func (sidecar *Sidecar) Destroy() {
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

		err := sidecar.cloud.Exec(
			context.Background(),
			sidecar.container,
			cloud.ExecConfig{Cmd: cmd, AttachStderr: true, AttachStdout: true},
			sidecar.getLogger("rm"),
		)
		if err != nil {
			log.Errorf(
				err,
				"unable to cleanup sidecar directory: %s %s",
				sidecar.GetGitDir(),
				sidecar.hostSubDir,
			)
		}
	}

	log.Debugf(
		nil,
		"destroying sidecar %s container",
		sidecar.container.String(),
	)

	err := sidecar.cloud.DestroyContainer(
		context.Background(),
		sidecar.container,
	)
	if err != nil {
		log.Errorf(
			err,
			"unable to destroy sidecar container: %s %s",
			sidecar.container.String(),
		)

		return
	}

	// wait for ssh-agent to die in order to avoid go routine leakage
	sidecar.sshAgent.Wait()
}
