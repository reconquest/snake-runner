package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/lineflushwriter-go"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/bufferer"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/env"
	"github.com/reconquest/snake-runner/internal/masker"
	"github.com/reconquest/snake-runner/internal/runner"
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
	config       config.Pipeline
	runnerConfig *runner.Config

	task        tasks.PipelineRun
	utilization chan *cloud.Container

	job snake.PipelineJob
	log *cog.Logger

	configJob config.Job `gonstructor:"-"`

	mutex     sync.Mutex       `gonstructor:"-"`
	container *cloud.Container `gonstructor:"-"`
	sidecar   *sidecar.Sidecar `gonstructor:"-"`
	shell     string           `gonstructor:"-"`
	env       *env.Env         `gonstructor:"-"`
	logs      struct {
		masker io.WriteCloser
		direct *bufferer.Bufferer
	} `gonstructor:"-"`
}

func (job *ProcessJob) init() {
	job.setupDirectLog()
}

func (process *ProcessJob) setupDirectLog() {
	process.logs.direct = bufferer.NewBufferer(
		bufferer.DefaultLogsBufferSize,
		bufferer.DefaultLogsBufferTimeout,
		func(buffer []byte) {
			err := process.client.PushLogs(
				process.task.Pipeline.ID,
				process.job.ID,
				string(buffer),
			)
			if err != nil {
				process.log.Errorf(
					err,
					"unable to push logs to remote server",
				)
			}
		},
	)

	go process.logs.direct.Run()
}

func (process *ProcessJob) setupMaskerLog(env *env.Env) {
	process.logs.masker = lineflushwriter.New(
		masker.NewMasker(env, process.task.EnvMask, process.logs.direct),
		&sync.Mutex{},
		true,
	)
}

func (process *ProcessJob) Destroy() {
	if process.logs.masker != nil {
		process.logs.masker.Close()
	} else if process.logs.direct != nil {
		process.logs.direct.Close()
	}

	if process.logs.direct != nil {
		process.logs.direct.Wait()
	}
}

func (process *ProcessJob) Run() error {
	var ok bool
	process.configJob, ok = process.config.Jobs[process.job.Name]
	if !ok {
		return process.directRemoteErrorf(
			nil,
			"unable to find given job %q in %q",
			process.job.Name,
			process.task.Pipeline.Filename,
		)
	}

	process.env = env.NewBuilder(
		process.task,
		process.task.Pipeline,
		process.job,
		process.config,
		process.configJob,
		process.runnerConfig,
		process.sidecar.GetGitDir(),
		process.sidecar.GetSshDir(),
	).Build()

	process.setupMaskerLog(process.env)

	imageExpr, image := process.getImage()

	process.log.Debugf(nil, "image: %s → %s", imageExpr, image)

	err := process.ensureImage(image)
	if err != nil {
		return process.maskRemoteErrorf(err, "unable to pull image %q", image)
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
		process.sidecar.GetContainerVolumes(),
	)
	if err != nil {
		return process.maskRemoteErrorf(err, "unable to create a container")
	}

	defer func() {
		process.utilization <- process.container
	}()

	err = process.detectShell()
	if err != nil {
		return process.maskRemoteErrorf(err, "unable to detect shell in container")
	}

	for _, command := range process.configJob.Commands {
		err = process.execShell(command)
		if err != nil {
			return process.maskRemoteErrorf(
				karma.
					Describe("cmd", command).
					Reason(err),
				"command failed",
			)
		}
	}

	return nil
}

func (process *ProcessJob) getImage() (string, string) {
	var image string
	switch {
	case process.configJob.Image != "":
		image = process.configJob.Image
	case process.config.Image != "":
		image = process.config.Image
	default:
		image = DefaultImage
	}

	expanded := process.expandEnv(image)

	return image, expanded
}

func (process *ProcessJob) expandEnv(target string) string {
	return os.Expand(target, func(name string) string {
		value, _ := process.env.Get(name)
		return value
	})
}

func (process *ProcessJob) maskRemoteLog(text string) {
	process.log.Debugf(nil, "%s", strings.TrimSpace(text))

	process.logs.masker.Write([]byte(text))
}

func (process *ProcessJob) directRemoteLog(text string) {
	process.log.Debugf(nil, "%s", strings.TrimSpace(text))

	process.logs.direct.Write([]byte(text))
}

func (process *ProcessJob) maskRemoteErrorf(
	reason error,
	format string,
	args ...interface{},
) error {
	err := karma.Format(reason, format, args...)
	process.logs.masker.Write([]byte("\n\n" + err.Error() + "\n"))
	return err
}

func (process *ProcessJob) directRemoteErrorf(
	reason error,
	format string,
	args ...interface{},
) error {
	err := karma.Format(reason, format, args...)
	process.logs.direct.Write([]byte("\n\n" + err.Error() + "\n"))
	return err
}

func (process *ProcessJob) execShell(cmd string) error {
	process.maskSendPrompt([]string{cmd})

	err := make(chan error, 1)
	go func() {
		defer audit.Go("exec", cmd)()

		err <- process.cloud.Exec(
			process.ctx,
			process.container,
			types.ExecConfig{
				Env:          process.env.GetAll(),
				WorkingDir:   process.sidecar.GetGitDir(),
				Cmd:          []string{process.shell, "-c", cmd},
				AttachStdout: true,
				AttachStderr: true,
			},
			process.maskRemoteLog,
		)
	}()

	select {
	case value := <-err:
		return value
	case <-process.ctx.Done():
		return context.Canceled
	}
}

func (process *ProcessJob) maskSendPrompt(cmd []string) {
	process.maskRemoteLog("\n$ " + strings.Join(cmd, " ") + "\n")
}

func (process *ProcessJob) directSendPrompt(cmd []string) {
	process.directRemoteLog("\n$ " + strings.Join(cmd, " ") + "\n")
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
		process.directRemoteLog(
			fmt.Sprintf("\n:: pulling docker image: %s\n", tag),
		)

		err := process.cloud.PullImage(process.ctx, tag, process.maskRemoteLog)
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

	process.directRemoteLog(
		fmt.Sprintf(
			"\n:: Using docker image: %s @ %s\n",
			strings.Join(image.RepoTags, ", "),
			image.ID,
		),
	)

	return nil
}

func (process *ProcessJob) withLock(fn func()) {
	process.mutex.Lock()
	defer process.mutex.Unlock()
	fn()
}
