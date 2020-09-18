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
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/env"
	"github.com/reconquest/snake-runner/internal/job"
	"github.com/reconquest/snake-runner/internal/ptr"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/sidecar"
	"github.com/reconquest/snake-runner/internal/signal"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/spawner"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/status"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/reconquest/snake-runner/internal/utils"
)

const (
	FAIL_ALL_JOBS = -1
)

//go:generate gonstructor -type Process
type Process struct {
	parentCtx    context.Context
	ctx          context.Context
	client       *api.Client
	runnerConfig *runner.Config
	task         tasks.PipelineRun
	spawner      spawner.Spawner
	log          *cog.Logger

	sshKey sshkey.Key

	status  status.Status   `gonstructor:"-"`
	sidecar sidecar.Sidecar `gonstructor:"-"`
	config  config.Pipeline `gonstructor:"-"`

	auth struct {
		variable    spawner.Auths `gonstructor:"-"`
		environment spawner.Auths `gonstructor:"-"`
	} `gonstructor:"-"`

	onceFail   sync.Once `gonstructor:"-"`
	configCond signal.Condition
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
		process.fail(FAIL_ALL_JOBS)

		return karma.Format(
			err,
			"unable to update pipeline status",
		)
	}

	err = process.parseVariables()
	if err != nil {
		process.status = status.FAILED
		process.fail(FAIL_ALL_JOBS)
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
		process.fail(FAIL_ALL_JOBS)

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

	status, jobErr := process.processJob(index, job)

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
	index int,
	target snake.PipelineJob,
) (result status.Status, err error) {
	var task *job.Process

	defer func() {
		tears := recover()
		if tears != nil {
			err = karma.Describe("panic", tears).
				Describe("stacktrace", string(debug.Stack())).
				Reason("PANIC")

			log.Error(err)

			result = status.FAILED

			if task != nil {
				task.ErrorfDirect(
					err,
					"the job failed due to an internal error, please report it",
				)
			}
		}

		if task != nil {
			task.Destroy()
		}
	}()

	task = job.NewProcess(
		process.ctx,
		process.spawner,
		process.client,
		process.runnerConfig,
		process.task,
		process.config,
		target,
		process.log.NewChildWithPrefix(
			fmt.Sprintf(
				"[pipeline:%d job:%d]",
				process.task.Pipeline.ID,
				target.ID,
			),
		),
		job.ContextSpawnerAuth{
			Runner:   process.runnerConfig.GetDockerAuthConfig(),
			Pipeline: process.auth.variable,
			Env:      process.auth.environment,
		},
	)

	// we want to read config only in the first container because users would
	// expect to see logs for git clone and other stuff in the first job in the
	// list instead of what job comes up first in a race
	if index == 1 {
		err = process.readConfig(task)
		if err != nil {
			return status.FAILED, task.ErrorfDirect(
				err,
				"unable to read config file",
			)
		}

		process.configCond.Satisfy()
	} else {
		process.configCond.Wait()
	}

	task.SetSidecar(process.sidecar)
	task.SetConfigPipeline(process.config)

	err = task.Run()
	if err != nil {
		if utils.IsCanceled(err) {
			// special case when runner gets terminated
			if utils.IsDone(process.parentCtx) {
				task.LogDirect("\n\nWARNING: snake-runner has been terminated")

				return status.FAILED, err
			}

			return status.CANCELED, err
		}

		return status.FAILED, err
	}

	return status.SUCCESS, nil
}

func (process *Process) readConfig(job *job.Process) error {
	process.sidecar = process.buildSidecar(job)

	err := process.sidecar.Serve(
		process.ctx,
		sidecar.ServeOptions{
			Env:        env.NewEnv(process.task.Env),
			KnownHosts: process.task.KnownHosts,
			CloneURL:   process.task.CloneURL.GetPreferredURL(),
			Commit:     process.task.Pipeline.Commit,
		},
	)
	if err != nil {
		return karma.Format(
			err,
			"unable ot start sidecar container with repository",
		)
	}

	yamlContents, err := process.sidecar.ReadFile(
		process.ctx,
		process.sidecar.GitDir(),
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
}

func (process *Process) buildSidecar(job *job.Process) sidecar.Sidecar {
	name := fmt.Sprintf(
		"pipeline-%d-uniq-%s", process.task.Pipeline.ID, utils.RandString(10),
	)

	slug := fmt.Sprintf(
		"%s/%s", process.task.Project.Key, process.task.Repository.Slug,
	)

	switch process.runnerConfig.Mode {
	case runner.RUNNER_MODE_DOCKER:
		var volumes []spawner.Volume

		for _, volume := range process.runnerConfig.Sidecar.Docker.Volumes {
			volumes = append(volumes, spawner.Volume(volume))
		}

		return sidecar.NewCloudSidecarBuilder().
			Spawner(process.spawner).
			Name(name).
			Slug(slug).
			PipelinesDir(process.runnerConfig.PipelinesDir).
			PromptConsumer(job.SendPromptDirect).
			OutputConsumer(job.LogDirect).
			SshKey(process.sshKey).
			Volumes(volumes).
			Build()
	case runner.RUNNER_MODE_SHELL:
		return sidecar.NewShellSidecarBuilder().
			Spawner(process.spawner).
			Name(name).
			Slug(slug).
			PipelinesDir(process.runnerConfig.PipelinesDir).
			PromptConsumer(job.SendPromptDirect).
			OutputConsumer(job.LogDirect).
			SshKey(process.sshKey).
			Build()

	default:
		panic("BUG: unexpected runner mode: " + process.runnerConfig.Mode)
	}
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
			case failedID == FAIL_ALL_JOBS:
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
			err := json.Unmarshal([]byte(raw), &process.auth.variable)
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
			err := json.Unmarshal([]byte(raw), &process.auth.environment)
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
