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
	DefaultImage = "alpine"
)

//go:generate gonstructor -type ProcessJob
type ProcessJob struct {
	ctx          context.Context
	cloud        *cloud.Cloud
	client       *Client
	config       *Config
	runnerConfig *RunnerConfig

	task        tasks.PipelineRun
	utilization chan *cloud.Container

	job snake.PipelineJob
	log *cog.Logger

	container *cloud.Container `gonstructor:"-"`
	sidecar   *sidecar.Sidecar `gonstructor:"-"`
	shell     string           `gonstructor:"-"`
	env       []string         `gonstructor:"-"`
}

func (process *ProcessJob) run() error {
	configJob, ok := process.config.Jobs[process.job.Name]
	if !ok {
		return fmt.Errorf(
			"unable to find given job %q in %s",
			process.job.Name,
			process.task.Pipeline.Filename,
		)
	}

	var image string
	switch {
	case configJob.Image != "":
		image = configJob.Image
	case process.config.Image != "":
		image = process.config.Image
	default:
		image = DefaultImage
	}

	err := process.ensureImage(image)
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

	err = process.detectShell(configJob)
	if err != nil {
		return karma.Format(err, "unable to detect shell in container")
	}

	for _, command := range configJob.Commands {
		err = process.execShell(command)
		if err != nil {
			return err
		}
	}

	return nil
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
	if len(process.env) == 0 {
		process.env = NewEnvBuilder(
			process.task,
			process.task.Pipeline,
			process.job,
			process.runnerConfig,
			process.sidecar.GetContainerDir(),
		).build()
	}
	return process.env
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
			Env:          process.getEnv(),
			WorkingDir:   process.sidecar.GetContainerDir(),
			Cmd:          []string{process.shell, "-c", cmd},
			AttachStdout: true,
			AttachStderr: true,
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

func (process *ProcessJob) detectShell(configJob ConfigJob) error {
	if process.config.Shell != "" {
		process.log.Debugf(
			nil,
			"using shell specified in pipeline spec: %q",
			process.config.Shell,
		)
		process.shell = process.config.Shell
		return nil
	}

	if configJob.Shell != "" {
		process.log.Debugf(
			nil,
			"using shell specified in job spec: %q",
			configJob.Shell,
		)
		process.shell = configJob.Shell
		return nil
	}

	output := ""
	callback := func(line string) error {
		process.log.Tracef(nil, "shelldetect: %q", line)

		line = strings.TrimSpace(line)
		if line == "" {
			return nil
		}

		if output == "" {
			output = line
		} else {
			output += "\n" + line
		}

		return nil
	}

	cmd := []string{"sh", "-c", DETECT_SHELL_COMMAND}

	err := process.cloud.Exec(
		process.ctx,
		process.container,
		types.ExecConfig{
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		},
		callback,
	)
	if err != nil {
		return karma.Format(err, "execution of shell detection script failed")
	}

	if output == "" {
		process.shell = "sh"
		process.log.Debugf(nil, "using default shell: %q", process.shell)
	} else {
		process.shell = output
		process.log.Debugf(nil, "using shell detected in container: %q", process.shell)
	}

	return nil
}

func (process *ProcessJob) ensureImage(tag string) error {
	image, err := process.cloud.GetImageWithTag(process.ctx, tag)
	if err != nil {
		return err
	}

	if image == nil {
		err := process.pushLogs(
			fmt.Sprintf("\n:: pulling docker image: %s\n", tag),
		)
		if err != nil {
			return karma.Format(
				err,
				"unable to push log message",
			)
		}

		err = process.cloud.PullImage(process.ctx, tag, process.pushLogs)
		if err != nil {
			return err
		}

		image, err = process.cloud.GetImageWithTag(process.ctx, tag)
		if err != nil {
			return err
		}

		if image == nil {
			return karma.Format(
				err,
				"the image not found after pulling: %s",
				tag,
			)
		}
	}

	err = process.pushLogs(
		fmt.Sprintf(
			"\n:: Using docker image: %s @ %s\n",
			strings.Join(image.RepoTags, ", "),
			image.ID,
		),
	)
	if err != nil {
		return karma.Format(err, "unable to push logs")
	}

	return nil
}
