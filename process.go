package main

import (
	"strconv"

	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
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

type Process struct {
	// there should be no whole Runner struct
	// should be some sort of Client that does what Runner.request() does
	// Runner is here because it works
	runner *Runner
	log    *cog.Logger
	task   *Task
	cloud  *Cloud
}

func (process *Process) run() error {
	process.log.Debugf(nil, "starting process")
	defer func() {
		process.log.Infof(nil, "process finished")
	}()

	err := process.updatePipeline(StatusRunning)
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

		err := process.updateJob(job.ID, StatusRunning)
		if err != nil {
			process.fail(-1)

			return karma.Format(
				err,
				"unable to update job",
			)
		}

		process.doJob(job)
	}

	return nil
}

func (process *Process) doJob(job PipelineJob) {
}

func (process *Process) fail(jobID int) {
	found := false
	for _, job := range process.task.Jobs {
		var status string

		// find a job that was a cause for failing and mark it as failed,
		// mark other jobs as skipped
		if !found {
			if job.ID == jobID {
				found = true

				status = StatusFailed
			} else {
				continue
			}
		} else {
			status = StatusSkipped
		}

		err := process.updateJob(jobID, status)
		if err != nil {
			process.log.Errorf(err, "unable to update job status to %q", status)
		}
	}

	err := process.updatePipeline(StatusFailed)
	if err != nil {
		process.log.Errorf(err, "unable to update pipeline status to %q", StatusFailed)
	}
}

func (process *Process) updatePipeline(status string) error {
	err := process.runner.request().
		PUT().
		Path("/gate/task/pipeline/" + strconv.Itoa(process.task.Pipeline.ID)).
		Payload(&RunnerTaskUpdateRequest{
			Status: status,
		}).
		Do()
	if err != nil {
		return err
	}

	return nil
}

func (process *Process) updateJob(jobID int, status string) error {
	err := process.runner.request().
		PUT().
		Path(
			"/gate/task" +
				"/pipeline/" + strconv.Itoa(process.task.Pipeline.ID) +
				"/jobs/" + strconv.Itoa(process.task.Pipeline.ID),
		).
		Payload(&RunnerTaskUpdateRequest{
			Status: status,
		}).
		Do()
	if err != nil {
		return err
	}

	return nil
}
