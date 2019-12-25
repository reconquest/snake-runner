package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

var SSHConfigWithoutVerification = base64.StdEncoding.EncodeToString([]byte(`
Host *
	StrictHostKeyChecking no
	UserKnownHostsFile /dev/null
`))

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

func (cloud *Cloud) PrepareContainer(container string, key string) error {
	readEncode := func(path string) (string, error) {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return "", err
		}

		return base64.StdEncoding.EncodeToString(data), nil
	}

	privateKeyEncoded, err := readEncode(key)
	if err != nil {
		return karma.Format(
			err,
			"unable to encode private key",
		)
	}

	publicKeyEncoded, err := readEncode(key + ".pub")
	if err != nil {
		return karma.Format(
			err,
			"unable to encode public key",
		)
	}

	systemCommands := [][]string{
		{
			// note about /sbin/ for apk
			"apk", "--update", "add", "--no-cache",
			"ca-certificates", "bash", "git", "openssh",
		},

		// there must be a ssh-agent and volume binding to SSH_SOCK to the
		// container
		{"adduser", "--shell", "/bin/bash", "--disabled-password", "ci"},
	}

	callback := func(text string) error {
		log.Debugf(
			nil,
			"%s: %s",
			container, strings.TrimRight(text, "\n"),
		)
		return nil
	}

	for _, cmd := range systemCommands {
		log.Debugf(nil, "%s: %v", container, cmd)

		err := cloud.exec(container, types.ExecConfig{
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		}, callback)
		if err != nil {
			return karma.Describe("cmd", cmd).
				Format(err, "command failed")
		}
	}

	userCommands := [][]string{
		{
			"mkdir", "/home/ci/.ssh",
		},
		{
			"sh", "-c",
			"echo '" + privateKeyEncoded + "' | base64 -d > ~/.ssh/id_rsa",
		},
		{
			"sh", "-c",
			"echo '" + publicKeyEncoded + "' | base64 -d > ~/.ssh/id_rsa.pub",
		},
		{
			"chmod", "0600", ".ssh/id_rsa", ".ssh/id_rsa.pub",
		},
		{
			"sh", "-c",
			"echo '" + SSHConfigWithoutVerification + "' | base64 -d > ~/.ssh/config",
		},
	}

	for _, cmd := range userCommands {
		log.Debugf(karma.Describe("container", container), "exec user: %v", cmd)

		err := cloud.Exec(container, "/home/ci/", cmd, callback)
		if err != nil {
			return karma.Describe("cmd", cmd).
				Format(err, "command failed")
		}
	}

	return nil
}

func (cloud *Cloud) CreateContainer(
	ctx context.Context,
	image string,
	containerName string,
) (string, error) {
	config := &container.Config{
		Image: image,
		Labels: map[string]string{
			ImageLabelKey: version,
		},
		// Env: []string{},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
		// StdinOnce:    true,
		Entrypoint: []string{"/bin/sh"},
	}

	hostConfig := &container.HostConfig{
		NetworkMode: "host",
	}

	created, err := cloud.client.ContainerCreate(
		ctx, config,
		hostConfig, nil, containerName,
	)
	if err != nil {
		return "", err
	}

	id := created.ID

	err = cloud.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return "", karma.Format(
			err,
			"unable to start created container",
		)
	}

	return id, nil
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

func (cloud *Cloud) Exec(
	container string,
	cwd string,
	command []string,
	callback func(string) error,
) error {
	err := cloud.exec(container, types.ExecConfig{
		Cmd:          command,
		Privileged:   false,
		AttachStdout: true,
		AttachStderr: true,
		// should not be hardcode on this level
		WorkingDir: cwd,
		User:       "ci",
		Env:        []string{},
	}, callback)
	if err != nil {
		return err
	}

	return nil
}

func (cloud *Cloud) exec(
	container string,
	config types.ExecConfig,
	callback func(string) error,
) error {
	exec, err := cloud.client.ContainerExecCreate(
		context.Background(), container, config,
	)
	if err != nil {
		return err
	}

	response, err := cloud.client.ContainerExecAttach(
		context.Background(), exec.ID,
		types.ExecStartCheck{},
	)
	if err != nil {
		return err
	}

	writer := logwriter{callback: callback}

	_, err = stdcopy.StdCopy(writer, writer, response.Reader)
	if err != nil {
		return err
	}

	info, err := cloud.client.ContainerExecInspect(context.Background(), exec.ID)
	if err != nil {
		return karma.Format(
			err,
			"unable to inspect container",
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
