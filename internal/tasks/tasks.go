package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/snake"
)

const (
	KIND_PIPELINE_RUN     = "pipeline_run"
	KIND_PIPELINE_CANCEL  = "pipeline_cancel"
	KIND_RUNNER_TERMINATE = "runner_terminate"
)

type CloneMethod string

const (
	CloneMethodDefault CloneMethod = ""
	CloneMethodSSH                 = "ssh"
	CloneMethodHTTP                = "http"
)

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
		return url.SSH
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
	KnownHosts  []responses.KnownHost  `json:"known_hosts"`
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
		KIND_PIPELINE_RUN:     &PipelineRun{},
		KIND_PIPELINE_CANCEL:  &PipelineCancel{},
		KIND_RUNNER_TERMINATE: &RunnerTerminate{},
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
