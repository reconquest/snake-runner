package requests

import (
	"time"

	"github.com/reconquest/snake-runner/internal/status"
)

//go:generate gonstructor -type TaskUpdate
type TaskUpdate struct {
	Status     status.Status `json:"status"`
	StartedAt  *time.Time    `json:"started_at"`
	FinishedAt *time.Time    `json:"finished_at"`
}

type LogsPush struct {
	Data string `json:"data"`
}

type Heartbeat struct {
	Version *string `json:"version,omitempty"`
}

//go:generate gonstructor -type RunnerRegister
type RunnerRegister struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

//go:generate gonstructor -type Task
type Task struct {
	RunningPipelines []int  `json:"running_pipelines"`
	QueryPipeline    bool   `json:"query_pipeline"`
	SSHKey           string `json:"ssh_key"`
}
