package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

const (
	ImageLabelKey = "io.reconquest.snake"
)

type Cloud struct {
	client *client.Client
}

type ContainerState struct {
	types.ContainerState
}

func (state *ContainerState) GetError() error {
	data := []string{}
	if state.ExitCode != 0 {
		data = append(data, fmt.Sprintf("exit code: %d", state.ExitCode))
	}
	if state.Error != "" {
		data = append(data, fmt.Sprintf("error: %s", state.Error))
	}
	if state.OOMKilled {
		data = append(data, "killed by oom")
	}
	if len(data) > 0 {
		return fmt.Errorf("%s", strings.Join(data, "; "))
	}

	return nil
}

func NewCloud() (*Cloud, error) {
	var err error

	cloud := &Cloud{}
	cloud.client, err = client.NewEnvClient()
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to initialize docker client",
		)
	}

	return cloud, err
}

func (cloud *Cloud) CreateContainer(
	image string,
	containerName string,
) (string, error) {
	config := &container.Config{
		Image: image,
		Labels: map[string]string{
			ImageLabelKey: version,
		},
		Tty: true,
		//Env: []string{},
		AttachStdout: true,
		AttachStderr: true,
	}

	hostConfig := &container.HostConfig{}

	created, err := cloud.client.ContainerCreate(
		context.Background(), config,
		hostConfig, nil, containerName,
	)
	if err != nil {
		return "", err
	}

	return created.ID, nil
}

func (cloud *Cloud) WaitContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	wait, _ := cloud.client.ContainerWait(
		ctx, name,
		container.WaitConditionNotRunning,
	)

	select {
	case <-wait:
		break
	case <-ctx.Done():
		break
	}
}

func (cloud *Cloud) FollowLogs(container string, send func(string)) error {
	reader, err := cloud.client.ContainerLogs(
		context.Background(), container, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       "all",
		},
	)
	if err != nil {
		return err
	}

	defer reader.Close()

	buffer := make([]byte, 1024)
	for {
		size, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		send(string(buffer[:size]))
	}

	return nil
}

func (cloud *Cloud) StartContainer(container string) error {
	err := cloud.client.ContainerStart(
		context.Background(), container,
		types.ContainerStartOptions{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (cloud *Cloud) InspectContainer(container string) (*ContainerState, error) {
	response, err := cloud.client.ContainerInspect(context.Background(), container)
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to inspect container",
		)
	}

	return &ContainerState{*response.State}, nil
}

func (cloud *Cloud) DestroyContainer(container string) error {
	err := cloud.client.ContainerRemove(
		context.Background(), container,
		types.ContainerRemoveOptions{
			Force: true,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (cloud *Cloud) Exec(container string, command []string) error {
	exec, err := cloud.client.ContainerExecCreate(
		context.Background(), container,
		types.ExecConfig{
			Cmd: command,
		},
	)
	if err != nil {
		return err
	}

	err = cloud.client.ContainerExecStart(
		context.Background(), exec.ID,
		types.ExecStartCheck{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (cloud *Cloud) Cleanup() error {
	options := types.ContainerListOptions{}

	containers, err := cloud.client.ContainerList(
		context.Background(),
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

			err := cloud.DestroyContainer(container.ID)
			if err != nil {
				return karma.Describe("id", container.ID).Format(
					err,
					"unable to destroy container",
				)
			}

			destroyed++
		}
	}

	log.Infof(nil, "cleanup: destroyed %d containers", destroyed)

	return nil
}
