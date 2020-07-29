package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/snake"
)

const (
	KindPipelineRun     = "pipeline_run"
	KindPipelineCancel  = "pipeline_cancel"
	KindRunnerTerminate = "runner_terminate"
)

type PipelineRun struct {
	Pipeline    snake.Pipeline         `json:"pipeline"`
	Jobs        []snake.PipelineJob    `json:"jobs"`
	Env         map[string]string      `json:"env"`
	EnvMask     []string               `json:"env_mask"`
	Repository  responses.Repository   `json:"repository"`
	Project     responses.Project      `json:"project"`
	PullRequest *responses.PullRequest `json:"pull_request"`
	CloneURL    struct {
		SSH string `json:"ssh"`
	} `json:"clone_url"`
}

type PipelineCancel struct {
	Pipelines []int `json:"pipelines"`
}

type RunnerTerminate struct {
	Reason string `json:"reason"`
}

func Unmarshal(task responses.Task) (interface{}, error) {
	if task.Kind == "" {
		return nil, nil
	}

	log.Debugf(nil, "task kind: %s", task.Kind)

	kinds := map[string]interface{}{
		KindPipelineRun:     &PipelineRun{},
		KindPipelineCancel:  &PipelineCancel{},
		KindRunnerTerminate: &RunnerTerminate{},
	}

	if result, ok := kinds[task.Kind]; ok {
		err := json.Unmarshal(task.Data, result)
		if err != nil {
			return nil, err
		}

		log.Debugf(nil, "task: %#v", result)

		return result, nil
	} else {
		return nil, fmt.Errorf("unexpected task kind: %q", task.Kind)
	}
}
