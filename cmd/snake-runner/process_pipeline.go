package main

import (
	"context"
	"strconv"
	"time"

	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/ptr"
	"github.com/reconquest/snake-runner/internal/requests"
	"github.com/reconquest/snake-runner/internal/snake"
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

const (
	DefaultContainerCWD = "/home/ci/"
)

//go:generate gonstructor -type ProcessPipeline
type ProcessPipeline struct {
	// there should be no whole Runner struct
	// should be some sort of Client that does what Runner.request() does
	// Runner is here because it works
	requester   Requester
	sshKey      string
	task        tasks.PipelineRun
	cloud       *cloud.Cloud
	log         *cog.Logger
	ctx         context.Context
	utilization chan *cloud.Container

	status string
}

func (process *ProcessPipeline) run() error {
	process.log.Infof(nil, "pipeline started")

	defer func() {
		process.log.Infof(nil, "pipeline finished: status="+process.status)
	}()

	err := process.updatePipeline(
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

	total := len(process.task.Jobs)
	for index, job := range process.task.Jobs {
		process.log.Infof(nil, "%d/%d starting job: id=%d", index+1, total, job.ID)

		err := process.updateJob(job.ID, StatusRunning, ptr.TimePtr(utils.Now()), nil)
		if err != nil {
			process.fail(job.ID)

			return karma.Format(
				err,
				"unable to update job",
			)
		}

		err = process.runJob(job)
		if err != nil {
			if !karma.Contains(err, context.Canceled) {
				process.log.Infof(
					nil,
					"%d/%d finished job: id=%d status=%s",
					index+1, total, job.ID, StatusFailed,
				)

				process.fail(job.ID)
			}

			return err
		}

		process.log.Infof(
			nil,
			"%d/%d finished job: id=%d status=%s",
			index+1, total, job.ID, StatusSuccess,
		)

		err = process.updateJob(job.ID, StatusSuccess, nil, ptr.TimePtr(utils.Now()))
		if err != nil {
			process.fail(job.ID)

			return karma.Format(
				err,
				"unable to update job status to success, although job finished successfully",
			)
		}
	}

	err = process.updatePipeline(StatusSuccess, nil, ptr.TimePtr(utils.Now()))
	if err != nil {
		process.fail(-1)

		return karma.Format(
			err,
			"unable to update pipeline status",
		)
	}

	return nil
}

func (process *ProcessPipeline) runJob(job snake.PipelineJob) error {
	subprocess := &ProcessJob{
		cloud: process.cloud,

		requester:   process.requester,
		ctx:         process.ctx,
		cwd:         DefaultContainerCWD,
		task:        process.task,
		sshKey:      process.sshKey,
		job:         job,
		log:         process.log,
		utilization: process.utilization,
	}

	return subprocess.run()
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

		process.log.Infof(nil, "updating job status: %d -> %s", job.ID, status)

		err := process.updateJob(job.ID, status, nil, finished)
		if err != nil {
			process.log.Errorf(err, "unable to update job status to %q", status)
		}
	}

	err := process.updatePipeline(StatusFailed, nil, now)
	if err != nil {
		process.log.Errorf(err, "unable to update pipeline status to %q", StatusFailed)
	}
}

func (process *ProcessPipeline) updatePipeline(
	status string,
	startedAt *time.Time,
	finishedAt *time.Time,
) error {
	process.status = status

	err := process.requester.request().
		PUT().
		Path("/gate/pipelines/" + strconv.Itoa(process.task.Pipeline.ID)).
		Payload(&requests.TaskUpdate{
			Status:     status,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
		}).
		Do()
	if err != nil {
		return err
	}

	return nil
}

func (process *ProcessPipeline) updateJob(
	jobID int,
	status string,
	startedAt *time.Time,
	finishedAt *time.Time,
) error {
	err := process.requester.request().
		PUT().
		Path(
			"/gate" +
				"/pipelines/" + strconv.Itoa(process.task.Pipeline.ID) +
				"/jobs/" + strconv.Itoa(jobID),
		).
		Payload(&requests.TaskUpdate{
			Status:     status,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
		}).
		Do()
	if err != nil {
		return err
	}

	return nil
}
