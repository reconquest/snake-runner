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

type CloneMethod string

const (
	CloneMethodDefault CloneMethod = ""
	CloneMethodSSH                 = "ssh"
	CloneMethodHTTP                = "http"
)

func (method *CloneMethod) UnmarshalJSON(data []byte) error {
	var value string

	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}

	switch CloneMethod(value) {
	case CloneMethodDefault:
	case CloneMethodSSH:
	case CloneMethodHTTP:
	default:
		return fmt.Errorf("unsupported clone method: %s", value)
	}

	*method = CloneMethod(value)

	return nil
}

type CloneURL struct {
	Method CloneMethod `json:"method"`
	SSH    string      `json:"ssh"`
	HTTP   string      `json:"http"`
}

func (url CloneURL) GetPreferredURL() string {
	switch url.Method {
	case CloneMethodDefault:
		fallthrough
	case CloneMethodSSH:
		return url.SSH
	case CloneMethodHTTP:
		if url.HTTP == "" {
			return url.SSH
		}
		return url.HTTP
	default:
		panic("unexpected clone method: " + string(url.Method))
	}
}

type PipelineRun struct {
	Pipeline    snake.Pipeline         `json:"pipeline"`
	Jobs        []snake.PipelineJob    `json:"jobs"`
	Env         map[string]string      `json:"env"`
	EnvMask     []string               `json:"env_mask"`
	Repository  responses.Repository   `json:"repository"`
	Project     responses.Project      `json:"project"`
	PullRequest *responses.PullRequest `json:"pull_request"`
	CloneURL    CloneURL               `json:"clone_url"`
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
