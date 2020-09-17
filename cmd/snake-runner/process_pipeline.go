package main

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
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/env"
	"github.com/reconquest/snake-runner/internal/ptr"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/sidecar"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/syncdo"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/reconquest/snake-runner/internal/utils"
)

type DockerAuthConfigs struct {
	Runner   cloud.DockerConfig
	Env      cloud.DockerConfig
	Pipeline cloud.DockerConfig
	Job      cloud.DockerConfig
}

const (
	FailAllJobs = -1
)

//go:generate gonstructor -type ProcessPipeline
type ProcessPipeline struct {
	parentCtx    context.Context
	ctx          context.Context
	client       *Client
	runnerConfig *runner.Config
	task         tasks.PipelineRun
	cloud        *cloud.Cloud
	log          *cog.Logger
	utilization  chan *cloud.Container

	sshKey sshkey.Key

	status      string           `gonstructor:"-"`
	sidecar     *sidecar.Sidecar `gonstructor:"-"`
	initSidecar syncdo.Action    `gonstructor:"-"`
	config      config.Pipeline  `gonstructor:"-"`

	variableDockerConfig cloud.DockerConfig `gonstructor:"-"`
	envDockerConfig      cloud.DockerConfig `gonstructor:"-"`
	onceFail             sync.Once          `gonstructor:"-"`
}

func (process *ProcessPipeline) run() error {
	defer process.destroy()

	process.log.Infof(nil, "pipeline started")

	defer func() {
		process.log.Infof(nil, "pipeline finished: status="+process.status)
	}()

	err := process.client.UpdatePipeline(
		process.task.Pipeline.ID,
		StatusRunning,
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
		process.status = StatusFailed
		process.fail(FailAllJobs)
		return err
	}

	process.status, err = process.runJobs()
	if err != nil {
		return err
	}

	err = process.client.UpdatePipeline(
		process.task.Pipeline.ID,
		StatusSuccess,
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

func (process *ProcessPipeline) splitJobs() [][]snake.PipelineJob {
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

func (process *ProcessPipeline) runJobs() (string, error) {
	var once sync.Once
	var resultStatus string
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

	return StatusSuccess, nil
}

func (process *ProcessPipeline) runJob(total, index int, job snake.PipelineJob) (string, error) {
	process.log.Infof(
		nil,
		"%d/%d starting job: id=%d",
		index, total, job.ID,
	)

	err := process.updateJob(
		job.ID,
		StatusRunning,
		ptr.TimePtr(utils.Now()),
		nil,
	)
	if err != nil {
		return StatusFailed, karma.Format(
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

func (process *ProcessPipeline) processJob(target snake.PipelineJob) (status string, err error) {
	defer func() {
		tears := recover()
		if tears != nil {
			err = karma.Describe("panic", tears).
				Describe("stacktrace", string(debug.Stack())).
				Reason("PANIC")

			log.Error(err)

			status = StatusFailed
		}
	}()

	job := NewProcessJob(
		process.ctx,
		process.cloud,
		process.client,
		process.config,
		process.runnerConfig,
		process.task,
		process.utilization,
		target,
		process.log.NewChildWithPrefix(
			fmt.Sprintf(
				"[pipeline:%d job:%d]",
				process.task.Pipeline.ID,
				target.ID,
			),
		),
		DockerAuthConfigs{
			Runner:   process.runnerConfig.GetDockerAuthConfig(),
			Pipeline: process.variableDockerConfig,
			Env:      process.envDockerConfig,
		},
	)

	defer job.Destroy()

	err = process.readConfig(job)
	if err != nil {
		return StatusFailed, job.directRemoteErrorf(
			err,
			"unable to read config file",
		)
	}

	job.sidecar = process.sidecar
	job.config = process.config

	err = job.Run()
	if err != nil {
		if utils.IsCanceled(err) {
			// special case when runner gets terminated
			if utils.IsDone(process.parentCtx) {
				job.directRemoteLog("\n\nWARNING: snake-runner has been terminated")

				return StatusFailed, err
			}

			return StatusCanceled, err
		}

		return StatusFailed, err
	}

	return StatusSuccess, nil
}

func (process *ProcessPipeline) readConfig(job *ProcessJob) error {
	return process.initSidecar.Do(func() error {
		job.setupMaskWriter(env.NewEnv(process.task.Env))

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
			PromptConsumer(job.maskSendPrompt).
			OutputConsumer(job.maskRemoteLog).
			SshKey(process.sshKey).
			Volumes(process.runnerConfig.Sidecar.Docker.Volumes).
			Build()

		err := process.sidecar.Serve(
			process.ctx,
			process.task.CloneURL.GetPreferredURL(),
			process.task.Pipeline.Commit,
		)
		if err != nil {
			return karma.Format(
				err,
				"unable ot start sidecar container with repository",
			)
		}

		yamlContents, err := process.cloud.Cat(
			process.ctx,
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

func (process *ProcessPipeline) fail(failedID int) {
	process.onceFail.Do(func() {
		now := ptr.TimePtr(utils.Now())

		var failedStage string
		var found bool

		for _, job := range process.task.Jobs {
			var status string
			var finished *time.Time

			switch {
			case failedID == FailAllJobs:
				status = StatusFailed
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
				status = StatusSkipped
			}

			err := process.updateJob(
				job.ID,
				status,
				nil,
				finished,
			)
			if err != nil {
				process.log.Errorf(err, "unable to update job status to %q", status)
			}
		}

		err := process.client.UpdatePipeline(
			process.task.Pipeline.ID,
			StatusFailed,
			nil,
			now,
		)
		if err != nil {
			process.log.Errorf(
				err,
				"unable to update pipeline status to %q",
				StatusFailed,
			)
		}
	})
}

func (process *ProcessPipeline) updateJob(
	id int,
	status string,
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

func (process *ProcessPipeline) parseVariables() error {
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

func (process *ProcessPipeline) destroy() {
	if process.sidecar != nil {
		process.sidecar.Destroy()
	}
}
