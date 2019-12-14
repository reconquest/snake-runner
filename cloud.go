package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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

	hostConfig := &container.HostConfig{}

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

func (cloud *Cloud) Prepare(container string, callback func(string) error) error {
	commands := [][]string{
		{
			"/bin/mkdir", "/ci/",
		},
		{
			// note about /sbin/ for apk
			"/sbin/apk", "--update", "add", "--no-cache",
			"ca-certificates", "bash", "git", "openssh",
		},
	}

	for _, cmd := range commands {
		log.Debugf(karma.Describe("container", container), "executing: %v", cmd)
		exitcode, err := cloud.exec(container, types.ExecConfig{
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		}, callback)
		if err != nil {
			return karma.Describe("cmd", cmd).
				Format(err, "command failed")
		}

		if exitcode > 0 {
			return karma.Describe("cmd", cmd).
				Format(err, "exitcode: %v", exitcode)
		}
	}

	return nil
}

func (cloud *Cloud) Exec(
	container string,
	command []string,
	callback func(string) error,
) error {
	code, err := cloud.exec(container, types.ExecConfig{
		Cmd:          command,
		Privileged:   false,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/ci/",
		Env:          []string{},
	}, callback)
	if err != nil {
		return err
	}
	if code > 0 {
		return karma.
			Describe("exitcode", code).
			Format(
				nil,
				"exitcode is greater than zero",
			)
	}

	return nil
}

func (cloud *Cloud) exec(
	container string,
	config types.ExecConfig,
	callback func(string) error,
) (int, error) {
	exec, err := cloud.client.ContainerExecCreate(
		context.Background(), container, config,
	)
	if err != nil {
		return 0, err
	}

	response, err := cloud.client.ContainerExecAttach(
		context.Background(), exec.ID,
		types.ExecStartCheck{},
	)
	if err != nil {
		return 0, err
	}

	writer := logwriter{callback: callback}

	_, err = stdcopy.StdCopy(writer, writer, response.Reader)
	if err != nil {
		return 0, err
	}

	info, err := cloud.client.ContainerExecInspect(context.Background(), exec.ID)
	if err != nil {
		return 0, karma.Format(
			err,
			"unable to inspect container",
		)
	}

	return info.ExitCode, nil
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

// waiter, err := cloud.client.ContainerAttach(ctx, id, types.ContainerAttachOptions{
//    Stderr: true,
//    Stdout: true,
//    Stdin:  true,
//    Stream: true,
//})

// outputErrCh := make(chan error)
// go func() {
//    scanner := bufio.NewScanner(waiter.Reader)
//    for scanner.Scan() {
//        pipe.Output <- scanner.Text()
//    }
//    outputErrCh <- scanner.Err()
//}()

// go func(writer io.WriteCloser) {
//    for {
//        data, ok := <-pipe.Input
//        if !ok {
//            writer.Close()
//            return
//        }

//        writer.Write([]byte(data))
//    }
//}(waiter.Conn)

// statusCh, waitErrCh := cloud.client.ContainerWait(ctx, id, container.WaitConditionNotRunning)
// select {
// case err := <-outputErrCh:
//    if err != nil {
//        return err
//    }
// case err := <-waitErrCh:
//    if err != nil {
//        return err
//    }
// case a := <-statusCh:
//    fmt.Fprintf(os.Stderr, "XXXXXX cloud.go:153 a: %#v\n", a)
//}
