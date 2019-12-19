package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

//go:generate gonstructor -type Process
type Process struct {
	// there should be no whole Runner struct
	// should be some sort of Client that does what Runner.request() does
	// Runner is here because it works
	runner *Runner
	log    *cog.Logger
	task   *Task
	cloud  *Cloud

	container string
	ctx       context.Context
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

		err = process.doJob(job)
		if err != nil {
			process.fail(-1)
			return err
		}
	}

	return nil
}

func (process *Process) doJob(job PipelineJob) error {
	err := process.makeContainer(job)
	if err != nil {
		return err
	}
	return nil
}

func (process *Process) makeContainer(job PipelineJob) error {
	uniq := time.Now().UnixNano()

	var err error
	process.container, err = process.cloud.CreateContainer(
		process.ctx,
		"alpine",
		fmt.Sprintf(
			"pipeline-%d-job-%d-uniq-%v",
			process.task.Pipeline.ID,
			job.ID,
			uniq,
		),
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to create a container",
		)
	}

	callback := func(text string) error {
		return process.pushLogs(job, text)
	}

	err = process.cloud.PrepareContainer(process.container, process.runner.config.SSHKey, callback)
	if err != nil {
		return err
	}

	cwd := "/home/ci/"
	exec := func(cmd ...string) error {
		return process.cloud.Exec(process.container, cwd, cmd, callback)
	}

	err = exec("git", "clone", process.task.CloneURL.SSH, "/home/ci/job")
	if err != nil {
		return err
	}

	cwd = "/home/ci/job"

	err = exec("git", "checkout", process.task.Pipeline.Commit)
	if err != nil {
		return err
	}

	yamlContents, err := process.readFile(cwd, process.task.Pipeline.Filename)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "XXXXXX process.go:132 yamlContents: %#v\n", yamlContents)

	return nil
}

func (process *Process) readFile(cwd, path string) (string, error) {
	data := ""
	callback := func(line string) error {
		if data == "" {
			data = line
		} else {
			data = data + "\n" + line
		}

		return nil
	}

	err := process.cloud.Exec(process.container, cwd, []string{"cat", path}, callback)
	if err != nil {
		return "", err
	}

	return data, nil
}

func (process *Process) pushLogs(job PipelineJob, text string) error {
	process.log.Debugf(nil, "%s", strings.TrimRight(text, "\n"))
	return nil
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
		Path("/gate/pipelines/" + strconv.Itoa(process.task.Pipeline.ID)).
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
			"/gate" +
				"/pipelines/" + strconv.Itoa(process.task.Pipeline.ID) +
				"/jobs/" + strconv.Itoa(jobID),
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
