package cloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/docker/cli/cli/trust"
	docker_reference "github.com/docker/distribution/reference"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/registry"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

type DockerConfig struct {
	Auths map[string]types.AuthConfig `json:"auths"`
}

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
	volumes []string
}

type (
	OutputConsumer func(string)
	PromptConsumer func([]string)
)

func NewDocker(network string, volumes []string) (*Cloud, error) {
	var err error

	cloud := &Cloud{}

	cloud.network = network
	cloud.volumes = volumes

	cloud.client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
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
	configs []DockerConfig,
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
		func(ctx context.Context, index *registrytypes.IndexInfo) types.AuthConfig {
			configKey := index.Name
			if index.Official {
				configKey = registry.IndexServer
			}

			var found types.AuthConfig
			for _, config := range configs {
				if len(config.Auths) > 0 {
					auth, ok := config.Auths[configKey]
					if ok {
						found = auth
						serverAddress = configKey
					}
				}
			}

			return found
		},
		distributionRef.String(),
	)
	if err != nil {
		return err
	}

	auth, err := cloud.encodeAuth(serverAddress, *imgRefAndAuth.AuthConfig())
	if err != nil {
		return err
	}

	pullOptions := types.ImagePullOptions{
		RegistryAuth: auth,
		PrivilegeFunc: func() (string, error) {
			return auth, nil
		},
	}

	reader, err := cloud.client.ImagePull(
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

func (cloud *Cloud) encodeAuth(serverAddress string, auth types.AuthConfig) (string, error) {
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
		Cmd:          []string{"sh"},
		Entrypoint:   []string{""},
	}

	hostConfig := &container.HostConfig{
		Binds: append(cloud.volumes, volumes...),
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
	exec, err := cloud.client.ContainerExecCreate(
		ctx,
		container.ID,
		config,
	)
	if err != nil {
		return err
	}

	response, err := cloud.client.ContainerExecAttach(
		ctx, exec.ID,
		types.ExecStartCheck{},
	)
	if err != nil {
		return err
	}

	writer := callbackWriter{ctx: ctx, callback: callback}

	_, err = stdcopy.StdCopy(writer, writer, response.Reader)
	if err != nil {
		return karma.Format(err, "unable to read stdout of exec/attach")
	}

	info, err := cloud.client.ContainerExecInspect(ctx, exec.ID)
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
	callback := func(line string) {
		if data == "" {
			data = line
		} else {
			data = data + "\n" + line
		}
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
