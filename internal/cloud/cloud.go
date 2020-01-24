package cloud

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

var SSHConfigWithoutVerification = `Host *
	StrictHostKeyChecking no
	UserKnownHostsFile /dev/null
`

const (
	ImageLabelKey = "io.reconquest.snake"
)

// Someday it will be an interface
type Cloud struct {
	client *client.Client

	network string

	pullLocks mapMutex
}

type (
	OutputConsumer  func(string) error
	CommandConsumer func([]string) error
)

func NewDocker(network string) (*Cloud, error) {
	var err error

	cloud := &Cloud{}

	cloud.network = network

	cloud.client, err = client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to initialize docker client",
		)
	}

	return cloud, err
}

func (cloud *Cloud) PullImage(
	ctx context.Context,
	reference string,
	callback OutputConsumer,
) error {
	mutex := cloud.pullLocks.get(reference)
	mutex.Lock()
	defer mutex.Unlock()

	if mutex.value {
		// while thread were waiting for Lock() another thread which had
		// Lock'ed the mutex already pulled the image
		return nil
	}

	reader, err := cloud.client.ImagePull(ctx, reference, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	logwriter := logwriter{callback: callback}

	termFd, isTerm := term.GetFdInfo(logwriter)

	err = jsonmessage.DisplayJSONMessagesStream(
		reader,
		logwriter,
		termFd,
		isTerm,
		nil,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to read docker pull output",
		)
	}

	mutex.value = true

	return nil
}

func (cloud *Cloud) ListImages(
	ctx context.Context,
) ([]types.ImageSummary, error) {
	images, err := cloud.client.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	return images, nil
}

func (cloud *Cloud) CreateContainer(
	ctx context.Context,
	image string,
	containerName string,
	volumes []string,
) (*Container, error) {
	config := &container.Config{
		Image: image,
		Labels: map[string]string{
			ImageLabelKey: "true",
		},
		// Env: []string{},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
	}

	hostConfig := &container.HostConfig{
		Binds: volumes,
	}

	if cloud.network != "" {
		hostConfig.NetworkMode = container.NetworkMode(cloud.network)
	}

	created, err := cloud.client.ContainerCreate(
		ctx, config,
		hostConfig, nil, containerName,
	)
	if err != nil {
		return nil, err
	}

	id := created.ID

	err = cloud.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to start created container",
		)
	}

	return &Container{ID: id, Name: containerName}, nil
}

func (cloud *Cloud) InspectContainer(
	ctx context.Context,
	container *Container,
) (*ContainerState, error) {
	response, err := cloud.client.ContainerInspect(ctx, container.ID)
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to inspect container",
		)
	}

	return &ContainerState{*response.State}, nil
}

func (cloud *Cloud) DestroyContainer(
	ctx context.Context,
	container *Container,
) error {
	if container == nil {
		return nil
	}

	err := cloud.client.ContainerRemove(
		ctx, container.ID,
		types.ContainerRemoveOptions{
			Force: true,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (cloud *Cloud) Exec(
	ctx context.Context,
	container *Container,
	config types.ExecConfig,
	callback OutputConsumer,
) error {
	facts := karma.Describe("cmd", config.Cmd)

	exec, err := cloud.client.ContainerExecCreate(
		ctx,
		container.ID,
		config,
	)
	if err != nil {
		return facts.Reason(err)
	}

	response, err := cloud.client.ContainerExecAttach(
		ctx, exec.ID,
		types.ExecStartCheck{},
	)
	if err != nil {
		return facts.Reason(err)
	}

	writer := logwriter{callback: callback}

	_, err = stdcopy.StdCopy(writer, writer, response.Reader)
	if err != nil {
		return facts.Format(err, "unable to read stdout of exec/attach")
	}

	info, err := cloud.client.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return facts.Format(
			err,
			"unable to inspect container/exec",
		)
	}
	if info.ExitCode > 0 {
		return facts.
			Describe("exitcode", info.ExitCode).
			Format(
				nil,
				"execution failed: exitcode is greater than zero",
			)
	}

	return nil
}

func (cloud *Cloud) Cleanup(ctx context.Context) error {
	options := types.ContainerListOptions{}

	containers, err := cloud.client.ContainerList(
		ctx,
		options,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to list containers",
		)
	}

	destroyed := 0
	for _, container := range containers {
		if _, ours := container.Labels[ImageLabelKey]; ours {
			log.Infof(
				nil,
				"cleanup: destroying container %q %q in status: %s",
				container.ID,
				container.Names,
				container.Status,
			)

			err := cloud.DestroyContainer(ctx, &Container{ID: container.ID})
			if err != nil {
				log.Errorf(
					karma.Describe("id", container.ID).
						Describe("name", container.Names).Reason(err),
					"unable to destroy container",
				)
			}

			destroyed++
		}
	}

	log.Infof(nil, "cleanup: destroyed %d containers", destroyed)

	return nil
}

func (cloud *Cloud) Cat(
	ctx context.Context,
	container *Container,
	cwd string,
	path string,
) (string, error) {
	data := ""
	callback := func(line string) error {
		if data == "" {
			data = line
		} else {
			data = data + "\n" + line
		}

		return nil
	}

	err := cloud.Exec(
		ctx,
		container,
		types.ExecConfig{
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

func (cloud *Cloud) GetImageWithTag(
	ctx context.Context,
	tag string,
) (*types.ImageSummary, error) {
	images, err := cloud.ListImages(ctx)
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to list images",
		)
	}

	for _, image := range images {
		for _, repoTag := range image.RepoTags {
			if repoTag == tag ||
				(!strings.Contains(tag, ":") && strings.HasPrefix(repoTag, tag+":")) {
				return &image, nil
			}
		}
	}

	return nil, nil
}