package main

import (
	"time"
)

type RunnerTaskUpdateRequest struct {
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}
