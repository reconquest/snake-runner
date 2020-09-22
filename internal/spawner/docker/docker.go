package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/cli/cli/trust"
	docker_reference "github.com/docker/distribution/reference"
	docker_types "github.com/docker/docker/api/types"
	docker_container "github.com/docker/docker/api/types/container"
	docker_registrytypes "github.com/docker/docker/api/types/registry"
	docker_client "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/registry"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/spawner"
	"github.com/reconquest/snake-runner/internal/utils"
)

const (
	IMAGE_LABEL_KEY = "io.reconquest.snake"
)

type Container struct {
	id   string
	name string
}

func (container Container) ID() string {
	return container.id
}

func (container Container) String() string {
	return container.name
}

type Image struct {
	ID   string
	Tags []string
}

type Docker struct {
	client *docker_client.Client

	network string
	volumes []string
}

func NewDocker(network string, volumes []string) (*Docker, error) {
	var err error

	docker := &Docker{}

	docker.network = network
	docker.volumes = volumes

	docker.client, err = docker_client.NewClientWithOpts(docker_client.FromEnv,
		docker_client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to initialize docker client",
		)
	}

	return docker, err
}

func (docker *Docker) Type() spawner.SpawnerType {
	return spawner.SPAWNER_DOCKER
}

func (docker *Docker) PullImage(
	ctx context.Context,
	reference string,
	callback spawner.OutputConsumer,
	auths []spawner.Auths,
) error {
	distributionRef, err := docker_reference.ParseNormalizedNamed(reference)
	if err != nil {
		return karma.Format(err, "unable to parse ref: %s", reference)
	}
	if docker_reference.IsNameOnly(distributionRef) {
		distributionRef = docker_reference.TagNameOnly(distributionRef)
	}

	var serverAddress string
	imgRefAndAuth, err := trust.GetImageReferencesAndAuth(
		ctx,
		nil,
		func(_ context.Context, index *docker_registrytypes.IndexInfo) docker_types.AuthConfig {
			configKey := index.Name
			if index.Official {
				configKey = registry.IndexServer
			}

			var found spawner.AuthConfig
			for _, auth := range auths {
				if len(auth) > 0 {
					auth, ok := auth[configKey]
					if ok {
						found = auth
						serverAddress = configKey
					}
				}
			}

			return docker_types.AuthConfig{
				Username:      found.Username,
				Password:      found.Password,
				Auth:          found.Auth,
				ServerAddress: found.ServerAddress,
				IdentityToken: found.IdentityToken,
				RegistryToken: found.RegistryToken,
			}
		},
		distributionRef.String(),
	)
	if err != nil {
		return err
	}

	auth, err := docker.encodeAuth(serverAddress, *imgRefAndAuth.AuthConfig())
	if err != nil {
		return err
	}

	pullOptions := docker_types.ImagePullOptions{
		RegistryAuth: auth,
		PrivilegeFunc: func() (string, error) {
			return auth, nil
		},
	}

	reader, err := docker.client.ImagePull(
		ctx,
		distributionRef.String(),
		pullOptions,
	)
	if err != nil {
		return err
	}
	defer reader.Close()

	logwriter := callbackWriter{ctx: ctx, callback: callback}

	termFd, isTerm := term.GetFdInfo(logwriter)

	err = jsonmessage.DisplayJSONMessagesStream(
		reader, logwriter, termFd, isTerm, nil,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to read docker pull output",
		)
	}

	return nil
}

func (docker *Docker) ListImages(
	ctx context.Context,
) ([]docker_types.ImageSummary, error) {
	images, err := docker.client.ImageList(ctx, docker_types.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	return images, nil
}

func (docker *Docker) Create(
	ctx context.Context,
	opts spawner.CreateOptions,
) (spawner.Container, error) {
	config := &docker_container.Config{
		Image: opts.Image,
		Labels: map[string]string{
			IMAGE_LABEL_KEY: "true",
		},
		// Env: []string{},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
	}

	hostConfig := &docker_container.HostConfig{
		Binds: append([]string{}, docker.volumes...),
	}

	for _, vol := range opts.Volumes {
		hostConfig.Binds = append(hostConfig.Binds, string(vol))
	}

	if docker.network != "" {
		hostConfig.NetworkMode = docker_container.NetworkMode(docker.network)
	}

	created, err := docker.client.ContainerCreate(
		ctx, config,
		hostConfig, nil, opts.Name,
	)
	if err != nil {
		return nil, err
	}

	id := created.ID

	err = docker.client.ContainerStart(ctx, id, docker_types.ContainerStartOptions{})
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to start created container",
		)
	}

	return Container{id: id, name: opts.Name}, nil
}

func (docker *Docker) Destroy(
	ctx context.Context,
	container spawner.Container,
) error {
	if container == nil {
		return nil
	}

	err := docker.client.ContainerRemove(
		ctx, container.ID(),
		docker_types.ContainerRemoveOptions{
			Force: true,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (docker *Docker) Exec(
	ctx context.Context,
	container spawner.Container,
	opts spawner.ExecOptions,
) error {
	exec, err := docker.client.ContainerExecCreate(
		ctx,
		container.ID(),
		docker_types.ExecConfig{
			AttachStderr: opts.AttachStderr,
			AttachStdout: opts.AttachStdout,
			Env:          opts.Env,
			WorkingDir:   opts.WorkingDir,
			Cmd:          opts.Cmd,
		},
	)
	if err != nil {
		return err
	}

	response, err := docker.client.ContainerExecAttach(
		ctx, exec.ID,
		docker_types.ExecStartCheck{},
	)
	if err != nil {
		return err
	}

	writer := callbackWriter{ctx: ctx, callback: opts.OutputConsumer}

	_, err = stdcopy.StdCopy(writer, writer, response.Reader)
	if err != nil {
		return karma.Format(err, "unable to read stdout of exec/attach")
	}

	info, err := docker.client.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return karma.Format(
			err,
			"unable to inspect container/exec",
		)
	}
	if info.ExitCode > 0 {
		return karma.
			Describe("exitcode", info.ExitCode).
			Format(
				nil,
				"exitcode is greater than zero",
			)
	}

	return nil
}

func (docker *Docker) Cleanup() error {
	options := docker_types.ContainerListOptions{}

	containers, err := docker.client.ContainerList(context.Background(), options)
	if err != nil {
		return karma.Format(
			err,
			"unable to list containers",
		)
	}

	destroyed := 0
	for _, container := range containers {
		if _, ours := container.Labels[IMAGE_LABEL_KEY]; ours {
			log.Infof(
				nil,
				"cleanup: destroying container %q %q in status: %s",
				container.ID,
				container.Names,
				container.Status,
			)

			err := docker.Destroy(context.Background(), Container{id: container.ID})
			if err != nil {
				log.Errorf(
					karma.
						Describe("id", container.ID).
						Describe("name", container.Names).
						Reason(err),
					"unable to destroy container",
				)
			}

			destroyed++
		}
	}

	log.Infof(nil, "cleanup: destroyed %d containers", destroyed)

	return nil
}

func (docker *Docker) GetImageWithTag(
	ctx context.Context,
	tag string,
) (*Image, error) {
	images, err := docker.ListImages(ctx)
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
				return &Image{
					Tags: image.RepoTags,
					ID:   image.ID,
				}, nil
			}
		}
	}

	return nil, nil
}

func (docker *Docker) encodeAuth(serverAddress string, auth docker_types.AuthConfig) (string, error) {
	// https://github.com/docker/cli/blob/75ab44af6f20784b624419ce2df458f1b0322b26/cli/config/configfile/file.go#L106
	if auth.Auth != "" && auth.Username == "" && auth.Password == "" {
		decoded, err := base64.URLEncoding.DecodeString(auth.Auth)
		if err != nil {
			return "", karma.Format(
				err,
				"unable to decode 'auth' field as base64",
			)
		}

		chunks := strings.SplitN(string(decoded), ":", 2)
		if len(chunks) == 2 {
			auth.Auth = ""
			auth.Username = chunks[0]
			auth.Password = chunks[1]
			auth.ServerAddress = serverAddress
		}
	}

	if auth.Username == "" &&
		auth.Auth == "" &&
		auth.IdentityToken == "" &&
		auth.RegistryToken == "" &&
		auth.Password == "" &&
		auth.Email == "" {
		return "", nil
	}

	json, err := json.Marshal(auth)
	if err != nil {
		return "", karma.Format(
			err,
			"unable to encode docker auth config",
		)
	}

	return base64.URLEncoding.EncodeToString(json), nil
}

func (docker *Docker) Prepare(
	ctx context.Context,
	opts spawner.PrepareOptions,
) error {
	tag := opts.Image
	if !strings.Contains(tag, ":") {
		tag = tag + ":latest"
	}

	image, err := docker.GetImageWithTag(ctx, tag)
	if err != nil {
		return err
	}

	if image == nil {
		if opts.InfoConsumer != nil {
			opts.InfoConsumer(
				fmt.Sprintf("\n:: pulling docker image: %s\n", tag),
			)
		}

		err := docker.PullImage(ctx, tag, opts.OutputConsumer, opts.Auths)
		if err != nil {
			return err
		}

		image, err = docker.GetImageWithTag(ctx, tag)
		if err != nil {
			return karma.Format(err, "unable to get image after pulling")
		}

		if image == nil {
			return karma.Format(err, "image not found after pulling")
		}
	}

	if opts.InfoConsumer != nil {
		opts.InfoConsumer(
			fmt.Sprintf(
				"\n:: Using docker image: %s @ %s\n",
				strings.Join(image.Tags, ", "),
				image.ID,
			),
		)
	}

	return nil
}

type callbackWriter struct {
	ctx      context.Context
	callback spawner.OutputConsumer
}

func (callbackWriter callbackWriter) Write(data []byte) (int, error) {
	if callbackWriter.callback == nil {
		return len(data), nil
	}

	if utils.IsDone(callbackWriter.ctx) {
		return 0, context.Canceled
	}

	callbackWriter.callback(string(data))

	return len(data), nil
}

func (docker *Docker) DetectShell(
	ctx context.Context,
	container spawner.Container,
) (string, error) {
	output := ""
	callback := func(line string) {
		log.Tracef(nil, "shelldetect: %q", line)

		line = strings.TrimSpace(line)
		if line == "" {
			return
		}

		if output == "" {
			output = line
		} else {
			output += "\n" + line
		}
	}

	cmd := []string{
		DEFAULT_SHELL,
		DEFAULT_SHELL_FLAG_COMMAND,
		DEFAULT_DETECT_SHELL_COMMAND,
	}

	err := docker.Exec(
		ctx,
		container,
		spawner.ExecOptions{
			Cmd:            cmd,
			AttachStdout:   true,
			AttachStderr:   true,
			OutputConsumer: callback,
		},
	)
	if err != nil {
		return "", karma.Format(
			err,
			"execution of shell detection script failed",
		)
	}

	program := strings.TrimSpace(output)

	if program == "" {
		log.Tracef(nil, "shelldetect: using default shell: %q", DEFAULT_SHELL)

		return DEFAULT_SHELL, nil
	}

	log.Tracef(nil, "shelldetect: detected shell: %q", program)

	return program, nil
}
