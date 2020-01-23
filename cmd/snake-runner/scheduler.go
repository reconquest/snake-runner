package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/tasks"
)

func (runner *Runner) startScheduler() error {
	docker, err := cloud.NewDocker(runner.config.Docker.Network)
	if err != nil {
		return karma.Format(err, "unable to initialize container provider")
	}

	scheduler := &Scheduler{
		client:      runner.client,
		cloud:       docker,
		utilization: make(chan *cloud.Container, runner.config.MaxParallelPipelines*2),
		config:      runner.config,
	}

	err = docker.Cleanup(context.Background())
	if err != nil {
		return karma.Format(err, "unable to cleanup old containers")
	}

	log.Infof(nil, "task scheduler started")

	runner.scheduler = scheduler

	go scheduler.loop()
	go scheduler.utilize()

	return nil
}

//go:generate gonstructor -type Scheduler
type Scheduler struct {
	client       *Client
	cloud        *cloud.Cloud
	pipelinesMap sync.Map
	pipelines    int64
	cancels      sync.Map
	utilization  chan *cloud.Container
	config       *RunnerConfig

	sshKey *sshkey.Key `gonstructor:"-"`
}

func (scheduler *Scheduler) loop() {
	for {
		wait, err := scheduler.getAndServe()
		if err != nil {
			log.Error(err)
		}

		if wait {
			time.Sleep(scheduler.config.SchedulerInterval)
		}
	}
}

func (scheduler *Scheduler) getAndServe() (bool, error) {
	var err error

	if scheduler.sshKey == nil {
		scheduler.sshKey, err = sshkey.Generate()
		if err != nil {
			return true, karma.Format(err, "unable to generate ssh key")
		}
	}

	pipelines := atomic.LoadInt64(&scheduler.pipelines)

	log.Debugf(nil, "retrieving task")

	task, err := scheduler.client.GetTask(
		scheduler.getPipelines(),
		pipelines < scheduler.config.MaxParallelPipelines,
		scheduler.sshKey,
	)
	if err != nil || task != nil {
		defer func() {
			scheduler.sshKey = nil
		}()
	}

	switch {
	case err != nil:
		return true, karma.Format(err, "unable to get a task")

	case task == nil:
		return true, nil
	default:
		// pass sshkey by value and cause copying
		err = scheduler.serveTask(task, *scheduler.sshKey)
		if err != nil {
			return true, karma.Format(err, "unable to properly serve a task")
		}

		return false, nil
	}
}

func (scheduler *Scheduler) utilize() {
	for container := range scheduler.utilization {
		err := scheduler.cloud.DestroyContainer(context.Background(), container)
		if err != nil {
			log.Errorf(
				karma.Describe("id", container.ID).
					Describe("name", container.Name).
					Reason(err),
				"unable to utilize (destroy) container after a job",
			)
		}

		log.Debugf(nil, "container utilized: %s %s", container.ID, container.Name)
	}
}

func (scheduler *Scheduler) serveTask(task interface{}, sshKey sshkey.Key) error {
	switch task := task.(type) {
	case tasks.PipelineRun:
		atomic.AddInt64(&scheduler.pipelines, 1)

		go func() {
			defer atomic.AddInt64(&scheduler.pipelines, -1)

			err := scheduler.startPipeline(task, sshKey)
			if err != nil {
				log.Errorf(err, "an error occurred during task running")
			}
		}()

	case tasks.PipelineCancel:
		for _, id := range task.Pipelines {
			cancel, ok := scheduler.cancels.Load(id)
			if !ok {
				log.Warningf(
					nil,
					"unable to cancel pipeline %d, its context already gone",
					id,
				)
			} else {
				log.Infof(nil, "canceling pipeline: %d", id)
				cancel.(context.CancelFunc)()

				scheduler.cancels.Delete(id)
				scheduler.pipelinesMap.Delete(id)
			}
		}

	default:
		log.Errorf(nil, "unexpected type of task %#v: %T", task, task)
	}

	return nil
}

func (scheduler *Scheduler) startPipeline(
	task tasks.PipelineRun,
	sshKey sshkey.Key,
) error {
	log.Debugf(nil, "starting pipeline: %d", task.Pipeline.ID)

	ctx, cancel := context.WithCancel(context.Background())

	process := NewProcessPipeline(
		scheduler.client,
		scheduler.config,
		task,
		scheduler.cloud,
		log.NewChildWithPrefix(fmt.Sprintf("[pipeline:%d] ", task.Pipeline.ID)),
		ctx,
		scheduler.utilization,
		sshKey,
	)

	scheduler.pipelinesMap.Store(task.Pipeline.ID, struct{}{})
	defer scheduler.pipelinesMap.Delete(task.Pipeline.ID)

	scheduler.cancels.Store(task.Pipeline.ID, cancel)
	defer scheduler.cancels.Delete(task.Pipeline.ID)

	err := process.run()
	if err != nil {
		if karma.Contains(err, context.Canceled) {
			log.Infof(nil, "pipeline %d finished due to cancel", task.Pipeline.ID)
			return nil
		}

		return err
	}

	return nil
}

func (scheduler *Scheduler) getPipelines() []int {
	result := []int{}

	scheduler.pipelinesMap.Range(func(key interface{}, _ interface{}) bool {
		result = append(result, key.(int))
		return true
	})

	return result
}
