package sidecar

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/spawner"
	"github.com/reconquest/snake-runner/internal/sshkey"
)

//go:generate gonstructor -type CloudSidecar -constructorTypes builder

const (
	CLOUD_SIDECAR_IMAGE spawner.Image = "reconquest/snake-runner-sidecar"
)

var _ Sidecar = (*CloudSidecar)(nil)

var capabilitiesCloud = capabilities{
	//containers: true,
	volumes: true,
}

// CloudSidecar produces the following volumes:
// * volume with git repository cloned from the bitbucket instance
// * volume with shared ssh-agent socket
type CloudSidecar struct {
	spawner spawner.Spawner
	name    string
	// pipelinesDir is the directory on the host file system where all pipelines
	// and temporary stuff such as git repos and ssh sockets are stored
	pipelinesDir   string
	slug           string
	promptConsumer spawner.PromptConsumer
	outputConsumer spawner.OutputConsumer
	sshKey         sshkey.Key

	container spawner.Container `gonstructor:"-"`

	hostSubDir   string `gonstructor:"-"`
	containerDir string `gonstructor:"-"`

	// directory with git repository cloned
	gitDir string `gonstructor:"-"`

	// directory with ssh-agent socket
	sshDir    string `gonstructor:"-"`
	sshSocket string `gonstructor:"-"`

	sshAgent sync.WaitGroup
}

func (sidecar *CloudSidecar) Capabilities() Capabilities {
	return capabilitiesCloud
}

func (sidecar *CloudSidecar) ContainerVolumes() []spawner.Volume {
	return []spawner.Volume{
		spawner.Volume(sidecar.hostSubDir + "/" + SUBDIR_GIT + ":" + sidecar.gitDir),
		spawner.Volume(sidecar.hostSubDir + "/" + SUBDIR_SSH + ":" + sidecar.sshDir),
	}
}

func (sidecar *CloudSidecar) GitDir() string {
	return sidecar.gitDir
}

func (sidecar *CloudSidecar) SshSocketPath() string {
	return sidecar.sshDir
}

func (sidecar *CloudSidecar) Container() spawner.Container {
	return sidecar.container
}

func (sidecar *CloudSidecar) create(ctx context.Context) error {
	err := sidecar.spawner.Prepare(
		ctx,
		CLOUD_SIDECAR_IMAGE,
		sidecar.outputConsumer,
		spawner.DiscardConsumer,
		[]spawner.PullConfig{},
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to prepare sidecar",
		)
	}

	sidecar.hostSubDir = filepath.Join(sidecar.pipelinesDir, sidecar.name)
	sidecar.containerDir = filepath.Join("/pipeline")

	sidecar.gitDir = filepath.Join(sidecar.containerDir, SUBDIR_GIT, sidecar.slug)
	sidecar.sshDir = filepath.Join(sidecar.containerDir, SUBDIR_SSH)

	volumes := []spawner.Volume{
		spawner.Volume(sidecar.hostSubDir + ":" + sidecar.containerDir + ":rw"),

		// host dir is bound because we are going to get rid of entire sidecar
		// subdir during container destroying
		spawner.Volume(sidecar.pipelinesDir + ":/host:rw"),
	}

	sidecar.container, err = sidecar.spawner.Create(
		ctx,
		spawner.Name("snake-runner-sidecar-"+sidecar.name),
		CLOUD_SIDECAR_IMAGE,
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

func (sidecar *CloudSidecar) Serve(
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
	err = sidecar.spawner.Exec(ctx, sidecar.container, spawner.ExecConfig{
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
	sidecar.sshSocket, err = sidecar.startSshAgent(ctx)
	if err != nil {
		return karma.Format(
			err,
			"unable to start ssh-agent",
		)
	}

	env := []string{
		SSH_SOCKET_VAR + "=" + sidecar.sshSocket,
		"__SNAKE_PRIVATE_KEY=" + string(sidecar.sshKey.Private),
		"__SNAKE_SSH_CONFIG=" + SSH_CONFIG_NO_VERIFICATION,
	}

	// adding ssh key to the ssh-agent and tuning up git a bit
	basic := []string{
		`mkdir ~/.ssh`,
		`ssh-add -v - <<< "$__SNAKE_PRIVATE_KEY"`,
		`cat > ~/.ssh/config <<< "$__SNAKE_SSH_CONFIG"`,
		`git config --global advice.detachedHead false`,
	}

	cmd := []string{"bash", "-c", strings.Join(basic, " && ")}

	err = sidecar.spawner.Exec(ctx, sidecar.container, spawner.ExecConfig{
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

		err = sidecar.spawner.Exec(ctx, sidecar.container, spawner.ExecConfig{
			Env: []string{
				SSH_SOCKET_VAR + "=" + sidecar.sshSocket,
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

func (sidecar *CloudSidecar) startSshAgent(ctx context.Context) (string, error) {
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

		err := sidecar.spawner.Exec(
			context.Background(),
			sidecar.container,
			spawner.ExecConfig{Cmd: cmd, AttachStderr: true, AttachStdout: true},
			sidecar.getLogger("rm"),
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

	err := sidecar.spawner.Destroy(
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

func (sidecar *CloudSidecar) ReadFile(ctx context.Context, cwd, path string) (string, error) {
	data := ""
	callback := func(line string) {
		if data == "" {
			data = line
		} else {
			data = data + "\n" + line
		}
	}

	err := sidecar.spawner.Exec(
		ctx,
		sidecar.Container(),
		spawner.ExecConfig{
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          []string{"cat", path},
			WorkingDir:   cwd,
		},
		callback,
	)
	if err != nil {
		return "", err
	}

	return data, nil
}
