package main

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type Task struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

type TaskPipeline struct {
	Pipeline Pipeline      `json:"pipeline"`
	Jobs     []PipelineJob `json:"jobs"`
	CloneURL struct {
		SSH string `json:"ssh"`
	} `json:"clone_url"`
}

func (runner *Runner) getTask(job bool) (interface{}, error) {
	var response Task

	query := url.Values{}
	if job {
		query.Add("pipeline", "1")
	}

	err := runner.request().
		GET().
		Path("/gate/task?" + query.Encode()).
		Response(&response).
		Do()
	if err != nil {
		return nil, err
	}

	if response.Kind == "" {
		return nil, nil
	}

	switch response.Kind {
	case "pipeline_run":
		var task TaskPipeline
		err := json.Unmarshal(response.Data, &task)
		if err != nil {
			return nil, err
		}

		return task, nil
	default:
		return nil, fmt.Errorf("unexpected task kind: %q", response.Kind)
	}
}
