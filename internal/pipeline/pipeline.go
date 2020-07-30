package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/api"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/job"
	"github.com/reconquest/snake-runner/internal/ptr"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/sidecar"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/status"
	"github.com/reconquest/snake-runner/internal/syncdo"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/reconquest/snake-runner/internal/utils"
)

const (
	FailAllJobs = -1
)

//go:generate gonstructor -type Process
type Process struct {
	parentCtx    context.Context
	ctx          context.Context
	client       *api.Client
	runnerConfig *runner.Config
	task         tasks.PipelineRun
	cloud        cloud.Cloud
	log          *cog.Logger
	utilization  chan cloud.Container

	sshKey sshkey.Key

	status      status.Status    `gonstructor:"-"`
	sidecar     *sidecar.Sidecar `gonstructor:"-"`
	initSidecar syncdo.Action    `gonstructor:"-"`
	config      config.Pipeline  `gonstructor:"-"`

	variableDockerConfig cloud.PullConfig `gonstructor:"-"`
	envDockerConfig      cloud.PullConfig `gonstructor:"-"`
	onceFail             sync.Once        `gonstructor:"-"`
}

func (process *Process) Run() error {
	defer process.destroy()

	process.log.Infof(nil, "pipeline started")

	defer func() {
		process.log.Infof(nil, "pipeline finished: status=%s", process.status)
	}()

	err := process.client.UpdatePipeline(
		process.task.Pipeline.ID,
		status.RUNNING,
		ptr.TimePtr(utils.Now().UTC()),
		nil,
	)
	if err != nil {
		process.fail(FailAllJobs)

		return karma.Format(
			err,
			"unable to update pipeline status",
		)
	}

	err = process.parseVariables()
	if err != nil {
		process.status = status.FAILED
		process.fail(FailAllJobs)
		return err
	}

	process.status, err = process.runJobs()
	if err != nil {
		return err
	}

	err = process.client.UpdatePipeline(
		process.task.Pipeline.ID,
		status.SUCCESS,
		nil,
		ptr.TimePtr(utils.Now()),
	)
	if err != nil {
		process.fail(FailAllJobs)

		return karma.Format(
			err,
			"unable to update pipeline status",
		)
	}

	return nil
}

func (process *Process) splitJobs() [][]snake.PipelineJob {
	stages := []string{}
	for _, job := range process.task.Jobs {
		found := false
		for _, stage := range stages {
			if stage == job.Stage {
				found = true
				break
			}
		}
		if !found {
			stages = append(stages, job.Stage)
		}
	}

	result := [][]snake.PipelineJob{}
	for _, stage := range stages {
		stageJobs := []snake.PipelineJob{}

		for _, job := range process.task.Jobs {
			if job.Stage == stage {
				stageJobs = append(stageJobs, job)
			}
		}

		result = append(result, stageJobs)
	}

	return result
}

func (process *Process) runJobs() (status.Status, error) {
	var once sync.Once
	var resultStatus status.Status
	var resultErr error

	total := len(process.task.Jobs)
	index := 0
	for _, stageJobs := range process.splitJobs() {
		workers := &sync.WaitGroup{}

		for _, job := range stageJobs {
			index++
			workers.Add(1)
			go func(index int, job snake.PipelineJob) {
				defer audit.Go("job", index, job.ID)()
				defer workers.Done()

				status, err := process.runJob(total, index, job)
				if err != nil {
					once.Do(func() {
						resultStatus = status
						resultErr = err

						process.fail(job.ID)
					})
				}
			}(index, job)
		}

		workers.Wait()

		if resultErr != nil {
			return resultStatus, resultErr
		}
	}

	return status.SUCCESS, nil
}

func (process *Process) runJob(total, index int, job snake.PipelineJob) (status.Status, error) {
	process.log.Infof(
		nil,
		"%d/%d starting job: id=%d",
		index, total, job.ID,
	)

	err := process.updateJob(
		job.ID,
		status.RUNNING,
		ptr.TimePtr(utils.Now()),
		nil,
	)
	if err != nil {
		return status.FAILED, karma.Format(
			err,
			"unable to update job status",
		)
	}

	status, jobErr := process.processJob(job)

	process.log.Infof(
		nil,
		"%d/%d finished job: id=%d status=%s",
		index, total, job.ID, status,
	)

	updateErr := process.updateJob(
		job.ID,
		status,
		nil,
		ptr.TimePtr(utils.Now()),
	)
	if updateErr != nil {
		log.Errorf(
			updateErr,
			"unable to update job %d status to %s", job.ID, status,
		)
	}

	if jobErr != nil {
		return status, karma.Format(
			jobErr,
			"job=%d an error occurred during job running", job.ID,
		)
	}

	return status, nil
}

func (process *Process) processJob(
	target snake.PipelineJob,
) (result status.Status, err error) {
	defer func() {
		tears := recover()
		if tears != nil {
			err = karma.Describe("panic", tears).
				Describe("stacktrace", string(debug.Stack())).
				Reason("PANIC")

			log.Error(err)

			result = status.FAILED
		}
	}()

	job := job.NewProcess(
		process.ctx,
		process.cloud,
		process.client,
		process.runnerConfig,
		process.task,
		process.utilization,
		process.config,
		target,
		process.log.NewChildWithPrefix(
			fmt.Sprintf(
				"[pipeline:%d job:%d]",
				process.task.Pipeline.ID,
				target.ID,
			),
		),
		job.ContextPullConfig{
			Runner:   process.runnerConfig.GetDockerAuthConfig(),
			Pipeline: process.variableDockerConfig,
			Env:      process.envDockerConfig,
		},
	)

	defer job.Destroy()

	err = process.readConfig(job)
	if err != nil {
		return status.FAILED, job.ErrorfDirect(
			err,
			"unable to read config file",
		)
	}

	job.SetSidecar(process.sidecar)
	job.SetConfigPipeline(process.config)

	err = job.Run()
	if err != nil {
		if utils.IsCanceled(err) {
			// special case when runner gets terminated
			if utils.IsDone(process.parentCtx) {
				job.LogDirect("\n\nWARNING: snake-runner has been terminated")

				return status.FAILED, err
			}

			return status.CANCELED, err
		}

		return status.FAILED, err
	}

	return status.SUCCESS, nil
}

func (process *Process) readConfig(job *job.Process) error {
	return process.initSidecar.Do(func() error {
		process.sidecar = sidecar.NewSidecarBuilder().
			Cloud(process.cloud).
			Name(
				fmt.Sprintf(
					"pipeline-%d-uniq-%s",
					process.task.Pipeline.ID,
					utils.RandString(10),
				),
			).
			Slug(
				fmt.Sprintf(
					"%s/%s",
					process.task.Project.Key,
					process.task.Repository.Slug,
				),
			).
			PipelinesDir(process.runnerConfig.PipelinesDir).
			PromptConsumer(job.SendPromptDirect).
			OutputConsumer(job.LogDirect).
			SshKey(process.sshKey).
			Build()

		err := process.sidecar.Serve(
			process.ctx,
			process.task.CloneURL.SSH,
			process.task.Pipeline.Commit,
		)
		if err != nil {
			return karma.Format(
				err,
				"unable ot start sidecar container with repository",
			)
		}

		yamlContents, err := process.cat(
			process.sidecar.GetContainer(),
			process.sidecar.GetGitDir(),
			process.task.Pipeline.Filename,
		)
		if err != nil {
			return karma.Format(
				err,
				"unable to obtain file from sidecar container with repository: %q",
				process.task.Pipeline.Filename,
			)
		}

		process.config, err = config.Unmarshal([]byte(yamlContents))
		if err != nil {
			return karma.Format(
				err,
				"unable to unmarshal yaml data: %q",
				process.task.Pipeline.Filename,
			)
		}

		return nil
	})
}

func (process *Process) fail(failedID int) {
	process.onceFail.Do(func() {
		now := ptr.TimePtr(utils.Now())

		var failedStage string
		var found bool

		for _, job := range process.task.Jobs {
			var result status.Status
			var finished *time.Time

			switch {
			case failedID == FailAllJobs:
				result = status.FAILED
				finished = now

			case job.ID == failedID:
				found = true
				failedStage = job.Stage
				continue

			case !found:
				continue

			case found && job.Stage == failedStage:
				continue

			case found:
				result = status.SKIPPED
			}

			err := process.updateJob(
				job.ID,
				result,
				nil,
				finished,
			)
			if err != nil {
				process.log.Errorf(
					err,
					"unable to update job status to %q",
					result,
				)
			}
		}

		err := process.client.UpdatePipeline(
			process.task.Pipeline.ID,
			status.FAILED,
			nil,
			now,
		)
		if err != nil {
			process.log.Errorf(
				err,
				"unable to update pipeline status to %q",
				status.FAILED,
			)
		}
	})
}

func (process *Process) updateJob(
	id int,
	status status.Status,
	startedAt *time.Time,
	finishedAt *time.Time,
) error {
	process.log.Infof(nil, "updating job: id=%d → status=%s", id, status)

	return process.client.UpdateJob(
		process.task.Pipeline.ID,
		id,
		status,
		startedAt,
		finishedAt,
	)
}

func (process *Process) parseVariables() error {
	if process.config.Variables != nil {
		raw, ok := process.config.Variables["DOCKER_AUTH_CONFIG"]
		if ok {
			err := json.Unmarshal([]byte(raw), &process.variableDockerConfig)
			if err != nil {
				return karma.Format(
					err,
					"json: unable to decode DOCKER_AUTH_CONFIG "+
						"specified on the pipeline level",
				)
			}
		}
	}

	if process.task.Env != nil {
		raw, ok := process.task.Env["DOCKER_AUTH_CONFIG"]
		if ok {
			err := json.Unmarshal([]byte(raw), &process.envDockerConfig)
			if err != nil {
				return karma.Format(
					err,
					"json: unable to decode DOCKER_AUTH_CONFIG "+
						"specified as a pipeline environment variable",
				)
			}
		}
	}

	return nil
}

func (process *Process) destroy() {
	if process.sidecar != nil {
		process.sidecar.Destroy()
	}
}

func (process *Process) cat(
	container cloud.Container,
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

	err := process.cloud.Exec(
		process.ctx,
		container,
		cloud.ExecConfig{
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
