package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/snake"
)

func Unmarshal(task responses.Task) (interface{}, error) {
	if task.Kind == "" {
		return nil, nil
	}

	log.Debugf(nil, "task kind: %s", task.Kind)

	switch task.Kind {
	case KindPipelineRun:
		var result PipelineRun
		err := json.Unmarshal(task.Data, &result)
		if err != nil {
			return nil, err
		}

		log.Debugf(nil, "task: %#v", result)

		return result, nil

	case "pipeline_cancel":
		var result PipelineCancel
		err := json.Unmarshal(task.Data, &result)
		if err != nil {
			return nil, err
		}

		log.Debugf(nil, "task: %#v", result)

		return result, nil

	default:
		return nil, fmt.Errorf("unexpected task kind: %q", task.Kind)
	}
}

const (
	KindPipelineRun    = "pipeline_run"
	KindPipelineCancel = "pipeline_cancel"
)

type PipelineRun struct {
	Pipeline snake.Pipeline      `json:"pipeline"`
	Jobs     []snake.PipelineJob `json:"jobs"`
	CloneURL struct {
		SSH string `json:"ssh"`
	} `json:"clone_url"`

	Env map[string]string `json:"env"`
}

type PipelineCancel struct {
	Pipelines []int `json:"pipelines"`
}
