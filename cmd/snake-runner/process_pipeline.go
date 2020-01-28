package main

import (
	"context"
	"fmt"
	"time"

	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/ptr"
	"github.com/reconquest/snake-runner/internal/sidecar"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/reconquest/snake-runner/internal/utils"
)

const (
	StatusPending  = "PENDING"
	StatusQueued   = "QUEUED"
	StatusRunning  = "RUNNING"
	StatusSuccess  = "SUCCESS"
	StatusFailed   = "FAILED"
	StatusCanceled = "CANCELED"
	StatusSkipped  = "SKIPPED"
	StatusUnknown  = "UNKNOWN"
)

//go:generate gonstructor -type ProcessPipeline
type ProcessPipeline struct {
	// there should be no whole Runner struct
	// should be some sort of Client that does what Runner.request() does
	// Runner is here because it works
	client       *Client
	runnerConfig *RunnerConfig
	task         tasks.PipelineRun
	cloud        *cloud.Cloud
	log          *cog.Logger
	ctx          context.Context
	utilization  chan *cloud.Container

	status  string           `gonstructor:"-"`
	sidecar *sidecar.Sidecar `gonstructor:"-"`
	config  *Config          `gonstructor:"-"`

	sshKey sshkey.Key
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
		process.fail(-1)

		return karma.Format(
			err,
			"unable to update pipeline status",
		)
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
		process.fail(-1)

		return karma.Format(
			err,
			"unable to update pipeline status",
		)
	}

	return nil
}

func (process *ProcessPipeline) runJobs() (string, error) {
	total := len(process.task.Jobs)
	for index, job := range process.task.Jobs {
		process.log.Infof(
			nil,
			"%d/%d starting job: id=%d",
			index+1, total, job.ID,
		)

		status, err := process.runJob(job)

		if status == StatusFailed {
			process.fail(job.ID)
		}

		process.log.Infof(
			nil,
			"%d/%d finished job: id=%d status=%s",
			index+1, total, job.ID, status,
		)

		if err != nil {
			return status, err
		}

		err = process.client.UpdateJob(
			process.task.Pipeline.ID,
			job.ID,
			StatusSuccess,
			nil,
			ptr.TimePtr(utils.Now()),
		)
		if err != nil {
			process.fail(job.ID)

			return StatusFailed, karma.Format(
				err,
				"unable to update job status to success, but job finished successfully",
			)
		}
	}

	return StatusSuccess, nil
}

func (process *ProcessPipeline) runJob(job snake.PipelineJob) (string, error) {
	err := process.client.UpdateJob(
		process.task.Pipeline.ID,
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

	subprocess := NewProcessJob(
		process.ctx,
		process.cloud,
		process.client,
		process.config,
		process.runnerConfig,
		process.task,
		process.utilization,
		job,
		process.log,
	)

	if process.sidecar == nil {
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
			CommandConsumer(subprocess.sendPrompt).
			OutputConsumer(subprocess.pushLogs).
			SshKey(process.sshKey).
			Build()

		err := process.sidecar.Serve(
			process.ctx,
			process.task.CloneURL.SSH,
			process.task.Pipeline.Commit,
		)
		if err != nil {
			return StatusFailed, karma.Format(
				err,
				"unable ot start sidecar container",
			)
		}

		err = process.readConfig()
		if err != nil {
			return StatusFailed, err
		}
	}

	subprocess.sidecar = process.sidecar
	subprocess.config = process.config

	err = subprocess.run()
	if err != nil {
		if karma.Contains(err, context.Canceled) {
			return StatusCanceled, err
		}

		return StatusFailed, err
	}

	return StatusSuccess, nil
}

func (process *ProcessPipeline) fail(failedID int) {
	now := ptr.TimePtr(utils.Now())
	foundFailed := false

	for _, job := range process.task.Jobs {
		var status string

		var finished *time.Time
		switch {
		case failedID == -1:
			status = StatusFailed
			finished = now

		case job.ID == failedID:
			foundFailed = true
			status = StatusFailed
			finished = now

		case !foundFailed:
			continue

		case foundFailed:
			status = StatusSkipped
		}

		process.log.Infof(nil, "updating job: id=%d → status=%s", job.ID, status)

		err := process.client.UpdateJob(
			process.task.Pipeline.ID,
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
		process.log.Errorf(err, "unable to update pipeline status to %q", StatusFailed)
	}
}

func (process *ProcessPipeline) readConfig() error {
	yamlContents, err := process.cloud.Cat(
		process.ctx,
		process.sidecar.GetContainer(),
		process.sidecar.GetContainerDir(),
		process.task.Pipeline.Filename,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to read configuration file: %s",
			process.task.Pipeline.Filename,
		)
	}

	process.config, err = unmarshalConfig([]byte(yamlContents))
	if err != nil {
		return karma.Format(
			err,
			"unable to unmarshal configuration file: %s",
			process.task.Pipeline.Filename,
		)
	}

	return nil
}

func (process *ProcessPipeline) destroy() {
	if process.sidecar != nil {
		process.sidecar.Destroy()
	}
}
