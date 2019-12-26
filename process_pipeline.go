package main

import (
	"context"
	"strconv"
	"time"

	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/pkg/ptr"
	"github.com/reconquest/snake-runner/pkg/utils"
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
	requester Requester
	sshKey    string
	task      *Task
	cloud     *Cloud
	log       *cog.Logger
}

func (process *ProcessPipeline) run() error {
	process.log.Debugf(nil, "starting process")
	defer func() {
		process.log.Infof(nil, "process finished")
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
			process.fail(job.ID)
			return err
		}

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

func (process *ProcessPipeline) runJob(job PipelineJob) error {
	subprocess := &ProcessJob{
		cloud: process.cloud,

		requester: process.requester,
		ctx:       context.Background(),
		cwd:       DefaultContainerCWD,
		task:      process.task,
		sshKey:    process.sshKey,
		job:       job,
	}

	return subprocess.run()
}

func (process *ProcessPipeline) fail(failedID int) {
	foundFailed := false
	for _, job := range process.task.Jobs {
		var status string

		switch {
		case failedID == -1:
			status = StatusFailed

		case job.ID == failedID:
			foundFailed = true
			status = StatusFailed

		case !foundFailed:
			continue

		case foundFailed:
			status = StatusSkipped
		}

		process.log.Infof(nil, "updating job status: %d -> %s", job.ID, status)

		err := process.updateJob(job.ID, status, nil, nil)
		if err != nil {
			process.log.Errorf(err, "unable to update job status to %q", status)
		}
	}

	err := process.updatePipeline(StatusFailed, nil, nil)
	if err != nil {
		process.log.Errorf(err, "unable to update pipeline status to %q", StatusFailed)
	}
}

func (process *ProcessPipeline) updatePipeline(
	status string,
	startedAt *time.Time,
	finishedAt *time.Time,
) error {
	err := process.requester.request().
		PUT().
		Path("/gate/pipelines/" + strconv.Itoa(process.task.Pipeline.ID)).
		Payload(&RunnerTaskUpdateRequest{
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
		Payload(&RunnerTaskUpdateRequest{
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
