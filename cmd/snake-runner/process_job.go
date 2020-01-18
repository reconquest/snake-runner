package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/sidecar"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/reconquest/snake-runner/internal/utils"
)

const (
	DefaultImage = "alpine:latest"
)

//go:generate gonstructor -type ProcessJob
type ProcessJob struct {
	ctx    context.Context
	cloud  *cloud.Cloud
	client *Client
	config *Config

	task        tasks.PipelineRun
	utilization chan *cloud.Container

	job snake.PipelineJob
	log *cog.Logger

	container *cloud.Container `gonstructor:"-"`
	sidecar   *sidecar.Sidecar `gonstructor:"-"`
}

func (process *ProcessJob) pushLogs(text string) error {
	// here should be a channel with a sort of buffer
	process.log.Debugf(nil, "%s", strings.TrimSpace(text))

	err := process.client.PushLogs(
		process.task.Pipeline.ID,
		process.job.ID,
		text,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to push logs to remote server",
		)
	}

	return nil
}

func (process *ProcessJob) getEnv() []string {
	env := []string{}
	for key, value := range process.task.Env {
		env = append(env, key+"="+value)
	}
	return env
}

func (process *ProcessJob) execSystem(cmd ...string) error {
	err := process.sendPrompt(cmd)
	if err != nil {
		return err
	}

	err = process.cloud.Exec(
		process.ctx,
		process.container,
		types.ExecConfig{
			Env:        process.getEnv(),
			WorkingDir: process.sidecar.GetContainerDir(),
			Cmd:        cmd,
		},
		process.pushLogs,
	)
	if err != nil {
		return karma.Describe("cmd", fmt.Sprintf("%v", cmd)).
			Format(err, "command failed")
	}

	return nil
}

func (process *ProcessJob) execShell(cmd string) error {
	err := process.sendPrompt([]string{cmd})
	if err != nil {
		return err
	}

	err = process.cloud.Exec(
		process.ctx,
		process.container,
		types.ExecConfig{
			Env:        process.getEnv(),
			WorkingDir: process.sidecar.GetContainerDir(),
			Cmd:        []string{"/bin/sh", "-c", cmd},
		},
		process.pushLogs,
	)
	if err != nil {
		return karma.Describe("cmd", fmt.Sprintf("%v", cmd)).
			Format(err, "command failed")
	}

	return nil
}

func (process *ProcessJob) sendPrompt(cmd []string) error {
	return process.pushLogs("\n$ " + strings.Join(cmd, " ") + "\n")
}

func (process *ProcessJob) run() error {
	image := process.config.Image
	if image == "" {
		image = DefaultImage
	}

	err := process.pullImage(image)
	if err != nil {
		return karma.Format(
			err,
			"unable to pull image: %s", image,
		)
	}

	process.container, err = process.cloud.CreateContainer(
		process.ctx,
		image,
		fmt.Sprintf(
			"pipeline-%d-job-%d-uniq-%v",
			process.task.Pipeline.ID,
			process.job.ID,
			utils.RandString(8),
		),
		process.sidecar.GetPipelineVolumes(),
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to create a container",
		)
	}

	defer func() {
		process.utilization <- process.container
	}()

	configJob, ok := process.config.Jobs[process.job.Name]
	if !ok {
		return fmt.Errorf(
			"unable to find given job %q in %s",
			process.job.Name,
			process.task.Pipeline.Filename,
		)
	}

	for _, script := range configJob.Script {
		err = process.execShell(script)
		if err != nil {
			return err
		}
	}

	return nil
}

func (process *ProcessJob) pullImage(image string) error {
	err := process.sendPrompt([]string{"docker", "pull", image})
	if err != nil {
		return karma.Format(
			err,
			"unable to start docker pull",
		)
	}

	err = process.cloud.PullImage(process.ctx, image, process.pushLogs)
	if err != nil {
		return err
	}

	return nil
}
