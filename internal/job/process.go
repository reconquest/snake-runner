package job

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/lineflushwriter-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/api"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/bufferer"
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/env"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/masker"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/sidecar"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/reconquest/snake-runner/internal/utils"
)

const (
	DEFAULT_CONTAINER_JOB_IMAGE = "alpine:latest"
)

type ContextExecutorAuth struct {
	Runner   executor.Auths
	Env      executor.Auths
	Pipeline executor.Auths
	Job      executor.Auths
}

func (config *ContextExecutorAuth) List() []executor.Auths {
	return []executor.Auths{
		config.Runner,
		config.Env,
		config.Pipeline,
		config.Job,
	}
}

//go:generate gonstructor -type Process -init init
type Process struct {
	ctx          context.Context
	executor     executor.Executor
	client       *api.Client
	runnerConfig *runner.Config

	task tasks.PipelineRun

	configPipeline  config.Pipeline
	job             snake.PipelineJob
	log             *cog.Logger
	contextPullAuth ContextExecutorAuth

	configJob config.Job `gonstructor:"-"`

	mutex     sync.Mutex         `gonstructor:"-"`
	container executor.Container `gonstructor:"-"`
	sidecar   sidecar.Sidecar    `gonstructor:"-"`
	shell     string             `gonstructor:"-"`
	env       *env.Env           `gonstructor:"-"`
	logs      struct {
		masker       masker.Masker
		maskWriter   *lineflushwriter.Writer
		directWriter *bufferer.Bufferer
	} `gonstructor:"-"`
}

func (job *Process) init() {
	job.setupDirectWriter()
}

func (job *Process) SetSidecar(car sidecar.Sidecar) {
	job.sidecar = car
}

func (job *Process) SetConfigPipeline(config config.Pipeline) {
	job.configPipeline = config
}

func (process *Process) setupDirectWriter() {
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

func (process *Process) SetupMaskWriter(env *env.Env) {
	masker := masker.NewWriter(env, process.task.EnvMask, process.logs.directWriter)

	process.logs.masker = masker

	process.logs.maskWriter = lineflushwriter.New(
		masker,
		&sync.Mutex{},
		true,
	)
}

func (process *Process) Destroy() {
	if process.logs.maskWriter != nil {
		process.logs.maskWriter.Close()
	}

	if process.logs.directWriter != nil {
		process.logs.directWriter.Close()
	}
}

func (process *Process) Run() error {
	var ok bool
	process.configJob, ok = process.configPipeline.Jobs[process.job.Name]
	if !ok {
		return process.ErrorfDirect(
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
		process.configPipeline,
		process.configJob,
		process.runnerConfig,
		process.sidecar.GitDir(),
		process.sidecar.SshSocketPath(),
		process.sidecar.SshKnownHostsPath(),
	).Build()

	process.SetupMaskWriter(process.env)

	imageExpr, image := process.getImage()

	process.log.Debugf(nil, "image: %s → %s", imageExpr, image)

	jobDockerConfig, err := process.getDockerAuthConfig()
	if err != nil {
		return process.errorRemote(err)
	}

	process.contextPullAuth.Job = jobDockerConfig

	process.log.Tracef(
		karma.
			Describe(
				"runner",
				fmt.Sprintf("%d items", len(process.contextPullAuth.Runner)),
			).
			Describe(
				"env",
				fmt.Sprintf("%d items", len(process.contextPullAuth.Env)),
			).
			Describe(
				"pipeline",
				fmt.Sprintf("%d items", len(process.contextPullAuth.Pipeline)),
			).
			Describe(
				"job",
				fmt.Sprintf("%d items", len(process.contextPullAuth.Job)),
			),
		"docker auth configs",
	)

	err = process.executor.Prepare(
		process.ctx,
		executor.PrepareOptions{
			Image:          image,
			OutputConsumer: process.LogMask,
			InfoConsumer:   process.LogMask,
			Auths:          process.contextPullAuth.List(),
		},
	)
	if err != nil {
		return process.errorfRemote(err, "unable to pull image %q", image)
	}

	process.container, err = process.executor.Create(
		process.ctx,
		executor.CreateOptions{
			Name: fmt.Sprintf(
				"pipeline-%d-job-%d-uniq-%v",
				process.task.Pipeline.ID,
				process.job.ID,
				utils.RandString(8),
			),
			Image:   image,
			Volumes: process.sidecar.ContainerVolumes(),
		},
	)
	if err != nil {
		return process.errorfRemote(err, "unable to create a container")
	}

	defer func() {
		err := process.executor.Destroy(context.Background(), process.container)
		if err != nil {
			log.Errorf(
				karma.
					Describe("id", process.container.ID).
					Describe("container", process.container.String()).
					Reason(err),
				"unable to destroy container",
			)
		}

		log.Debugf(
			nil,
			"container utilized: %s %s",
			process.container.ID(),
			process.container.String(),
		)
	}()

	err = process.detectShell()
	if err != nil {
		return process.errorfRemote(err, "unable to detect shell in container")
	}

	for _, command := range process.configJob.Commands {
		err = process.execShell(command)
		if err != nil {
			return process.errorfRemote(
				karma.
					Describe("cmd", command).
					Reason(err),
				"command failed",
			)
		}
	}

	return nil
}

func (process *Process) getImage() (string, string) {
	var image string
	switch {
	case process.configJob.Image != "":
		image = process.configJob.Image
	case process.configPipeline.Image != "":
		image = process.configPipeline.Image
	default:
		image = DEFAULT_CONTAINER_JOB_IMAGE
	}

	expanded := process.expandEnv(image)

	return image, expanded
}

func (process *Process) getDockerAuthConfig() (executor.Auths, error) {
	if process.configJob.Variables != nil {
		raw, ok := process.configJob.Variables["DOCKER_AUTH_CONFIG"]
		if ok {
			var cfg executor.Auths
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

	return executor.Auths{}, nil
}

func (process *Process) expandEnv(target string) string {
	return os.Expand(target, func(name string) string {
		value, _ := process.env.Get(name)
		return value
	})
}

func (process *Process) LogMask(text string) {
	process.log.Debugf(nil, "%s", strings.TrimSpace(process.logs.masker.Mask(text)))

	process.logs.maskWriter.Write([]byte(text))
}

func (process *Process) LogDirect(text string) {
	process.log.Debugf(nil, "%s", strings.TrimSpace(text))

	process.logs.directWriter.Write([]byte(text))
}

func (process *Process) errorRemote(err error) error {
	process.logs.maskWriter.Write([]byte("\n\n" + err.Error() + "\n"))
	return err
}

func (process *Process) errorfRemote(
	reason error,
	format string,
	args ...interface{},
) error {
	err := karma.Format(reason, format, args...)
	process.logs.maskWriter.Write([]byte("\n\n" + err.Error() + "\n"))
	return err
}

func (process *Process) ErrorfDirect(
	reason error,
	format string,
	args ...interface{},
) error {
	err := karma.Format(reason, format, args...)
	process.logs.directWriter.Write([]byte("\n\n" + err.Error() + "\n"))
	return err
}

func (process *Process) execShell(cmd string) error {
	process.MaskSendPrompt([]string{cmd})

	err := make(chan error, 1)
	go func() {
		defer audit.Go("exec", cmd)()

		err <- process.executor.Exec(
			process.ctx,
			process.container,
			executor.ExecOptions{
				Env:            process.env.GetAll(),
				WorkingDir:     process.sidecar.GitDir(),
				Cmd:            []string{process.shell, "-c", cmd},
				AttachStdout:   true,
				AttachStderr:   true,
				OutputConsumer: process.LogMask,
			},
		)
	}()

	select {
	case value := <-err:
		return value
	case <-process.ctx.Done():
		return context.Canceled
	}
}

func (process *Process) MaskSendPrompt(cmd []string) {
	process.LogMask("\n$ " + strings.Join(cmd, " ") + "\n")
}

func (process *Process) SendPromptDirect(cmd []string) {
	process.LogDirect("\n$ " + strings.Join(cmd, " ") + "\n")
}

func (process *Process) detectShell() error {
	if process.configPipeline.Shell != "" {
		process.log.Debugf(
			nil,
			"using shell specified in pipeline spec: %q",
			process.configPipeline.Shell,
		)
		process.shell = process.configPipeline.Shell
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

	var err error
	process.shell, err = process.executor.DetectShell(
		process.ctx,
		process.container,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to detect shell",
		)
	}

	return nil
}
