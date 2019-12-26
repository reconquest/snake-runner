package main

type runnerHeartbeatRequest struct {
	Pipelines []int `json:"pipelines,omitempty"`
}
