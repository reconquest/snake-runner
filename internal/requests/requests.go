package requests

import "time"

//go:generate gonstructor -type TaskUpdate
type TaskUpdate struct {
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type LogsPush struct {
	Data string `json:"data"`
}

type Heartbeat struct {
	//
}

//go:generate gonstructor -type RunnerRegister
type RunnerRegister struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}
