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

//go:generate gonstructor -type ProcessJob -init init
type ProcessJob struct {
	ctx          context.Context
	cloud        *cloud.Cloud
	client       *Client
	config       Config
	runnerConfig *RunnerConfig

	task        tasks.PipelineRun
	utilization chan *cloud.Container

	job snake.PipelineJob
	log *cog.Logger

	configJob ConfigJob `gonstructor:"-"`

	container  *cloud.Container    `gonstructor:"-"`
	sidecar    *sidecar.Sidecar    `gonstructor:"-"`
	shell      string              `gonstructor:"-"`
	env        []string            `gonstructor:"-"`
	logsWriter *LogsBufferedWriter `gonstructor:"-"`
}

func (process *ProcessJob) init() {
	process.logsWriter = NewLogsBufferedWriter(
		DefaultLogsBufferSize,
		DefaultLogsBufferTimeout,
		func(text string) {
			err := process.client.PushLogs(
				process.task.Pipeline.ID,
				process.job.ID,
				text,
			)
			if err != nil {
				process.log.Errorf(
					err,
					"unable to push logs to remote server",
				)
			}
		})

	go process.logsWriter.Run()
}

func (process *ProcessJob) destroy() {
	process.logsWriter.Close()
	process.logsWriter.Wait()
}

func (process *ProcessJob) run() error {
	var ok bool
	process.configJob, ok = process.config.Jobs[process.job.Name]
	if !ok {
		return process.remoteErrorf(
			nil,
			"unable to find given job %q in %q",
			process.job.Name,
			process.task.Pipeline.Filename,
		)
	}

	var image string
	switch {
	case process.configJob.Image != "":
		image = process.configJob.Image
	case process.config.Image != "":
		image = process.config.Image
	default:
		image = DefaultImage
	}

	err := process.ensureImage(image)
	if err != nil {
		return process.remoteErrorf(err, "unable to pull image %q", image)
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
		return process.remoteErrorf(err, "unable to create a container")
	}

	defer func() {
		process.utilization <- process.container
	}()

	err = process.detectShell()
	if err != nil {
		return process.remoteErrorf(err, "unable to detect shell in container")
	}

	for _, command := range process.configJob.Commands {
		err = process.execShell(command)
		if err != nil {
			return process.remoteErrorf(
				karma.
					Describe("cmd", command).
					Reason(err),
				"command failed",
			)
		}
	}

	return nil
}

func (process *ProcessJob) remoteLog(text string) {
	process.log.Debugf(nil, "%s", strings.TrimSpace(text))
	process.logsWriter.Write(text)
}

func (process *ProcessJob) remoteErrorf(
	reason error,
	format string,
	args ...interface{},
) error {
	err := karma.Format(reason, format, args...)
	process.logsWriter.Write("\n\n" + err.Error())
	return err
}

func (process *ProcessJob) getEnv() []string {
	if len(process.env) == 0 {
		process.env = NewEnvBuilder(
			process.task,
			process.task.Pipeline,
			process.job,
			process.config,
			process.configJob,
			process.runnerConfig,
			process.sidecar.GetContainerDir(),
		).Build()
	}

	return process.env
}

func (process *ProcessJob) execShell(cmd string) error {
	process.sendPrompt([]string{cmd})

	err := process.cloud.Exec(
		process.ctx,
		process.container,
		types.ExecConfig{
			Env:          process.getEnv(),
			WorkingDir:   process.sidecar.GetContainerDir(),
			Cmd:          []string{process.shell, "-c", cmd},
			AttachStdout: true,
			AttachStderr: true,
		},
		process.remoteLog,
	)

	return err
}

func (process *ProcessJob) sendPrompt(cmd []string) {
	process.remoteLog("\n$ " + strings.Join(cmd, " ") + "\n")
}

func (process *ProcessJob) detectShell() error {
	if process.config.Shell != "" {
		process.log.Debugf(
			nil,
			"using shell specified in pipeline spec: %q",
			process.config.Shell,
		)
		process.shell = process.config.Shell
		return nil
	}

	if process.configJob.Shell != "" {
		process.log.Debugf(
			nil,
			"using shell specified in job spec: %q",
			process.configJob.Shell,
		)
		process.shell = process.configJob.Shell
		return nil
	}

	output := ""
	callback := func(line string) {
		process.log.Tracef(nil, "shelldetect: %q", line)

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
		return karma.Format(
			err,
			"execution of shell detection script failed",
		)
	}

	if output == "" {
		process.shell = "sh"

		process.log.Debugf(nil, "using default shell: %q", process.shell)
	} else {
		process.shell = output

		process.log.Debugf(
			nil,
			"using shell detected in container: %q",
			process.shell,
		)
	}

	return nil
}

func (process *ProcessJob) ensureImage(tag string) error {
	if !strings.Contains(tag, ":") {
		tag = tag + ":latest"
	}

	image, err := process.cloud.GetImageWithTag(process.ctx, tag)
	if err != nil {
		return err
	}

	if image == nil {
		process.remoteLog(
			fmt.Sprintf("\n:: pulling docker image: %s\n", tag),
		)

		err := process.cloud.PullImage(process.ctx, tag, process.remoteLog)
		if err != nil {
			return err
		}

		image, err = process.cloud.GetImageWithTag(process.ctx, tag)
		if err != nil {
			return karma.Format(err, "unable to get image after pulling")
		}

		if image == nil {
			return karma.Format(err, "image not found after pulling")
		}
	}

	process.remoteLog(
		fmt.Sprintf(
			"\n:: Using docker image: %s @ %s\n",
			strings.Join(image.RepoTags, ", "),
			image.ID,
		),
	)

	return nil
}
