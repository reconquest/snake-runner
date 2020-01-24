// Code generated by gonstructor -type Scheduler; DO NOT EDIT.

package main

import (
	"sync"

	"github.com/reconquest/snake-runner/internal/cloud"
)

func NewScheduler(client *Client, cloud *cloud.Cloud, pipelinesMap sync.Map, pipelines int64, cancels sync.Map, utilization chan *cloud.Container, config *RunnerConfig) *Scheduler {
	return &Scheduler{client: client, cloud: cloud, pipelinesMap: pipelinesMap, pipelines: pipelines, cancels: cancels, utilization: utilization, config: config}
}