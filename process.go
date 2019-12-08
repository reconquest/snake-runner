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
}

func (process *Process) run() error {
	process.log.Debugf(nil, "starting process")
	defer func() {
		process.log.Infof(nil, "process finished")
	}()

	err := process.updatePipeline(StatusRunning)
	if err != nil {
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
			return karma.Format(
				err,
				"unable to update job",
			)

			// TODO: need to update other jobs and set them as failed due to an
			// error on this job
			// options:
			// * send sequential requests to update jobs
			// * send bulk update request
			// * send a special request and java does updates for all other jobs
		}
	}

	return nil
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
