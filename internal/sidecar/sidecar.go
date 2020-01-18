package sidecar

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/cloud"
)

const (
	SidecarImage = "reconquest/snake-runner-sidecar"
)

const SSHConfigWithoutVerification = `Host *
	StrictHostKeyChecking no
	UserKnownHostsFile /dev/null
`

//go:generate gonstructor -type Sidecar -constructorTypes builder
type Sidecar struct {
	cloud           *cloud.Cloud
	name            string
	pipelinesDir    string
	slug            string
	commandConsumer cloud.CommandConsumer
	outputConsumer  cloud.OutputConsumer
	sshKey          string

	container    *cloud.Container `gonstructor:"-"`
	containerDir string           `gonstructor:"-"`
	hostSubDir   string           `gonstructor:"-"`
}

func (sidecar *Sidecar) GetPipelineVolumes() []string {
	return []string{sidecar.hostSubDir + ":" + sidecar.containerDir + ":ro"}
}

func (sidecar *Sidecar) GetContainerDir() string {
	return sidecar.containerDir
}

func (sidecar *Sidecar) GetContainer() *cloud.Container {
	return sidecar.container
}

func (sidecar *Sidecar) create(ctx context.Context) error {
	images, err := sidecar.cloud.ListImages(ctx)
	if err != nil {
		return karma.Format(
			err,
			"unable to list existing images",
		)
	}

	found := false
	for _, image := range images {
		if image == SidecarImage || strings.HasPrefix(image, SidecarImage+":") {
			found = true
			break
		}
	}

	if !found {
		err := sidecar.cloud.PullImage(ctx, SidecarImage, sidecar.outputConsumer)
		if err != nil {
			return karma.Format(
				err,
				"unable to pull sidecar image: %s", SidecarImage,
			)
		}
	}

	sidecar.hostSubDir = filepath.Join(sidecar.pipelinesDir, sidecar.name)
	sidecar.containerDir = filepath.Join("/pipelines/", sidecar.slug)

	volumes := []string{
		sidecar.hostSubDir + ":" + sidecar.containerDir + ":rw",
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

func (sidecar *Sidecar) Serve(ctx context.Context, cloneURL string, commitish string) error {
	err := sidecar.create(ctx)
	if err != nil {
		return err
	}

	privateKey, err := ioutil.ReadFile(sidecar.sshKey)
	if err != nil {
		return karma.Format(
			err,
			"unable to read contents of ssh key",
		)
	}

	publicKey, err := ioutil.ReadFile(sidecar.sshKey + ".pub")
	if err != nil {
		return karma.Format(
			err,
			"unable to read contents of ssh key (.pub part)",
		)
	}

	env := []string{
		"__SNAKE_PRIVATE_KEY=" + string(privateKey),
		"__SNAKE_PUBLIC_KEY=" + string(publicKey),
		"__SNAKE_SSH_CONFIG=" + SSHConfigWithoutVerification,
	}

	basic := []string{
		`mkdir ~/.ssh`,
		`cat > ~/.ssh/id_rsa <<< "$__SNAKE_PRIVATE_KEY"`,
		`cat > ~/.ssh/id_rsa.pub <<< "$__SNAKE_PUBLIC_KEY"`,
		`cat > ~/.ssh/config <<< "$__SNAKE_SSH_CONFIG"`,
		`chmod 0600 ~/.ssh/id_rsa ~/.ssh/id_rsa.pub`,
		`git config --global advice.detachedHead false`,
	}

	cmd := []string{"bash", "-c", strings.Join(basic, " && ")}

	err = sidecar.cloud.Exec(ctx, sidecar.container, types.ExecConfig{
		Cmd:          cmd,
		Env:          env,
		AttachStdout: true,
		AttachStderr: true,
	}, sidecar.onlyLog)
	if err != nil {
		return karma.Format(
			err,
			"unable to prepare sidecar container",
		)
	}

	commands := [][]string{
		{`git`, `clone`, cloneURL, sidecar.containerDir},
		{`git`, `-C`, sidecar.containerDir, `checkout`, commitish},
	}

	for _, cmd := range commands {
		err = sidecar.commandConsumer(cmd)
		if err != nil {
			return err
		}

		err = sidecar.cloud.Exec(ctx, sidecar.container, types.ExecConfig{
			// NO ENV!
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		}, sidecar.outputConsumer)
		if err != nil {
			return karma.
				Describe("command", cmd).
				Format(
					err,
					"command failed",
				)
		}
	}

	return nil
}

func (sidecar *Sidecar) onlyLog(text string) error {
	log.Debugf(
		nil,
		"sidecar %s: %s",
		sidecar.container.Name, strings.TrimRight(text, "\n"),
	)
	return nil
}

func (sidecar *Sidecar) Destroy() {
	if sidecar.container == nil {
		return
	}
	// we use Background context here because local ctx can be destroyed
	// already

	if sidecar.name != "" {
		err := sidecar.cloud.Exec(context.Background(), sidecar.container, types.ExecConfig{
			Cmd:          []string{"rm", "-rf", filepath.Join("/host", sidecar.name)},
			AttachStderr: true,
			AttachStdout: true,
		}, sidecar.onlyLog)
		if err != nil {
			log.Errorf(
				err,
				"unable to cleanup sidecar directory: %s %s",
				sidecar.GetContainerDir(),
				sidecar.hostSubDir,
			)
		}
	}

	err := sidecar.cloud.DestroyContainer(context.Background(), sidecar.container)
	if err != nil {
		log.Errorf(
			err,
			"unable to destroy sidecar container: %s %s",
			sidecar.container.ID,
			sidecar.container.Name,
		)
	}
}
