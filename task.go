package main

import (
	"encoding/json"
	"fmt"

	"github.com/reconquest/pkg/log"
)

type Task struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

type TaskPipelineRun struct {
	Pipeline Pipeline      `json:"pipeline"`
	Jobs     []PipelineJob `json:"jobs"`
	CloneURL struct {
		SSH string `json:"ssh"`
	} `json:"clone_url"`

	Env map[string]string `json:"env"`
}

type TaskPipelineCancel struct {
	Pipelines []int `json:"pipelines"`
}

func (runner *Runner) getTask(queryPipeline bool) (interface{}, error) {
	var response Task

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

	if response.Kind == "" {
		return nil, nil
	}

	log.Debugf(nil, "task kind: %s", response.Kind)

	switch response.Kind {
	case "pipeline_run":
		var task TaskPipelineRun
		err := json.Unmarshal(response.Data, &task)
		if err != nil {
			return nil, err
		}

		log.Debugf(nil, "task: %#v", task)

		return task, nil

	case "pipeline_cancel":
		var task TaskPipelineCancel
		err := json.Unmarshal(response.Data, &task)
		if err != nil {
			return nil, err
		}

		log.Debugf(nil, "task: %#v", task)

		return task, nil

	default:
		return nil, fmt.Errorf("unexpected task kind: %q", response.Kind)
	}
}
