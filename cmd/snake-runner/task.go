package main

import (
	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/tasks"
)

func (runner *Runner) getTask(queryPipeline bool) (interface{}, error) {
	var response responses.Task

	payload := struct {
		RunningPipelines []int `json:"running_pipelines"`
		QueryPipeline    bool  `json:"query_pipeline"`
	}{
		RunningPipelines: runner.scheduler.getPipelines(),
		QueryPipeline:    queryPipeline,
	}

	err := runner.request().
		POST().
		Path("/gate/task").
		Payload(payload).
		Response(&response).
		Do()
	if err != nil {
		return nil, err
	}

	return tasks.Unmarshal(response)
}
