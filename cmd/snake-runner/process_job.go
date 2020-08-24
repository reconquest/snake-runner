package main

import (
	"context"
	"encoding/json"
	"fmt"
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

	job           snake.PipelineJob
	log           *cog.Logger
	dockerConfigs DockerAuthConfigs

	configJob config.Job `gonstructor:"-"`

	mutex     sync.Mutex       `gonstructor:"-"`
	container *cloud.Container `gonstructor:"-"`
	sidecar   *sidecar.Sidecar `gonstructor:"-"`
	shell     string           `gonstructor:"-"`
	env       *env.Env         `gonstructor:"-"`
	logs      struct {
		masker       masker.Masker
		maskWriter   *lineflushwriter.Writer
		directWriter *bufferer.Bufferer
	} `gonstructor:"-"`
}

func (job *ProcessJob) init() {
	job.setupDirectWriter()
}

func (process *ProcessJob) setupDirectWriter() {
	process.logs.directWriter = bufferer.NewBufferer(
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

	go process.logs.directWriter.Run()
}

func (process *ProcessJob) setupMaskWriter(env *env.Env) {
	masker := masker.NewWriter(env, process.task.EnvMask, process.logs.directWriter)

	process.logs.masker = masker

	process.logs.maskWriter = lineflushwriter.New(
		masker,
		&sync.Mutex{},
		true,
	)
}

func (process *ProcessJob) Destroy() {
	if process.logs.maskWriter != nil {
		process.logs.maskWriter.Close()
	} else if process.logs.directWriter != nil {
		process.logs.directWriter.Close()
	}

	if process.logs.directWriter != nil {
		process.logs.directWriter.Wait()
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

	process.setupMaskWriter(process.env)

	imageExpr, image := process.getImage()

	process.log.Debugf(nil, "image: %s → %s", imageExpr, image)

	jobDockerConfig, err := process.getDockerAuthConfig()
	if err != nil {
		return process.maskRemoteError(err)
	}

	process.dockerConfigs.Job = jobDockerConfig

	process.log.Tracef(
		karma.
			Describe(
				"runner",
				fmt.Sprintf("%d items", len(process.dockerConfigs.Runner.Auths)),
			).
			Describe(
				"env",
				fmt.Sprintf("%d items", len(process.dockerConfigs.Env.Auths)),
			).
			Describe(
				"pipeline",
				fmt.Sprintf("%d items", len(process.dockerConfigs.Pipeline.Auths)),
			).
			Describe(
				"job",
				fmt.Sprintf("%d items", len(process.dockerConfigs.Job.Auths)),
			),
		"docker auth configs",
	)

	err = process.ensureImage(image)
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

func (process *ProcessJob) getDockerAuthConfig() (cloud.DockerConfig, error) {
	if process.configJob.Variables != nil {
		raw, ok := process.configJob.Variables["DOCKER_AUTH_CONFIG"]
		if ok {
			var cfg cloud.DockerConfig
			err := json.Unmarshal([]byte(raw), &cfg)
			if err != nil {
				return cfg, karma.Format(
					err,
					"unable to decode DOCKER_AUTH_CONFIG "+
						"specified on the job level",
				)
			}

			return cfg, nil
		}
	}

	return cloud.DockerConfig{}, nil
}

func (process *ProcessJob) expandEnv(target string) string {
	return os.Expand(target, func(name string) string {
		value, _ := process.env.Get(name)
		return value
	})
}

func (process *ProcessJob) maskRemoteLog(text string) {
	process.log.Debugf(nil, "%s", strings.TrimSpace(process.logs.masker.Mask(text)))

	process.logs.maskWriter.Write([]byte(text))
}

func (process *ProcessJob) directRemoteLog(text string) {
	process.log.Debugf(nil, "%s", strings.TrimSpace(text))

	process.logs.directWriter.Write([]byte(text))
}

func (process *ProcessJob) maskRemoteError(err error) error {
	process.logs.maskWriter.Write([]byte("\n\n" + err.Error() + "\n"))
	return err
}

func (process *ProcessJob) maskRemoteErrorf(
	reason error,
	format string,
	args ...interface{},
) error {
	err := karma.Format(reason, format, args...)
	process.logs.maskWriter.Write([]byte("\n\n" + err.Error() + "\n"))
	return err
}

func (process *ProcessJob) directRemoteErrorf(
	reason error,
	format string,
	args ...interface{},
) error {
	err := karma.Format(reason, format, args...)
	process.logs.directWriter.Write([]byte("\n\n" + err.Error() + "\n"))
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

		err := process.cloud.PullImage(
			process.ctx,
			tag,
			process.maskRemoteLog,
			[]cloud.DockerConfig{
				process.dockerConfigs.Job,
				process.dockerConfigs.Pipeline,
				process.dockerConfigs.Env,
				process.dockerConfigs.Runner,
			},
		)
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
