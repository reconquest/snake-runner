package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/reconquest/snake-runner/internal/cloud"
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

func (docker *Docker) PullImage(
	ctx context.Context,
	reference string,
	callback cloud.OutputConsumer,
	configs []cloud.PullConfig,
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
		func(ctx context.Context, index *docker_registrytypes.IndexInfo) docker_types.AuthConfig {
			configKey := index.Name
			if index.Official {
				configKey = registry.IndexServer
			}

			var found cloud.AuthConfig
			for _, config := range configs {
				if len(config.Auths) > 0 {
					auth, ok := config.Auths[configKey]
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

func (docker *Docker) CreateContainer(
	ctx context.Context,
	image string,
	containerName string,
	volumes []string,
) (cloud.Container, error) {
	config := &docker_container.Config{
		Image: image,
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
		Binds: append(docker.volumes, volumes...),
	}

	if docker.network != "" {
		hostConfig.NetworkMode = docker_container.NetworkMode(docker.network)
	}

	created, err := docker.client.ContainerCreate(
		ctx, config,
		hostConfig, nil, containerName,
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

	return Container{id: id, name: containerName}, nil
}

//func (docker *Docker) InspectContainer(
//    ctx context.Context,
//    container docker.Container,
//) (*ContainerState, error) {
//    response, err := docker.client.ContainerInspect(ctx, container.ID)
//    if err != nil {
//        return nil, karma.Format(
//            err,
//            "unable to inspect container",
//        )
//    }

//    return &docker.ContainerState{*response.State}, nil
//}

func (docker *Docker) DestroyContainer(
	ctx context.Context,
	container cloud.Container,
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
	container cloud.Container,
	config cloud.ExecConfig,
	callback cloud.OutputConsumer,
) error {
	exec, err := docker.client.ContainerExecCreate(
		ctx,
		container.ID(),
		docker_types.ExecConfig{
			AttachStderr: config.AttachStderr,
			AttachStdout: config.AttachStdout,
			Env:          config.Env,
			WorkingDir:   config.WorkingDir,
			Cmd:          config.Cmd,
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

	writer := callbackWriter{ctx: ctx, callback: callback}

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

func (docker *Docker) Cleanup(ctx context.Context) error {
	options := docker_types.ContainerListOptions{}

	containers, err := docker.client.ContainerList(ctx, options)
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

			err := docker.DestroyContainer(ctx, Container{id: container.ID})
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

func (docker *Docker) GetImageWithTag(
	ctx context.Context,
	tag string,
) (*cloud.Image, error) {
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
				return &cloud.Image{
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

type callbackWriter struct {
	ctx      context.Context
	callback cloud.OutputConsumer
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
