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
	"github.com/reconquest/snake-runner/internal/tasks"
)

func (runner *Runner) startScheduler() error {
	docker, err := cloud.NewDocker()
	if err != nil {
		return karma.Format(err, "unable to initialize container provider")
	}

	scheduler := &Scheduler{
		maxPipelines: runner.config.MaxParallelPipelines,
		runner:       runner,
		cloud:        docker,
		utilization:  make(chan *cloud.Container, runner.config.MaxParallelPipelines*2),
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
	runner       *Runner
	cloud        *cloud.Cloud
	pipelinesMap sync.Map
	pipelines    int64
	maxPipelines int64
	cancels      sync.Map
	utilization  chan *cloud.Container
}

func (scheduler *Scheduler) loop() {
	for {
		pipelines := atomic.LoadInt64(&scheduler.pipelines)

		log.Debugf(nil, "retrieving task")

		task, err := scheduler.runner.getTask(pipelines < scheduler.maxPipelines)
		if err != nil {
			log.Errorf(err, "unable to get a task")
			time.Sleep(scheduler.runner.config.SchedulerInterval)
			continue
		}

		if task == nil {
			time.Sleep(scheduler.runner.config.SchedulerInterval)
			continue
		}

		err = scheduler.serveTask(task)
		if err != nil {
			log.Errorf(err, "unable to properly serve a task")
			time.Sleep(scheduler.runner.config.SchedulerInterval)
			continue
		}
	}
}

func (scheduler *Scheduler) utilize() {
	for container := range scheduler.utilization {
		err := scheduler.cloud.DestroyContainer(context.Background(), container.ID)
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

func (scheduler *Scheduler) serveTask(task interface{}) error {
	switch task := task.(type) {
	case tasks.PipelineRun:
		atomic.AddInt64(&scheduler.pipelines, 1)

		go func() {
			defer atomic.AddInt64(&scheduler.pipelines, -1)

			err := scheduler.startPipeline(task)
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

func (scheduler *Scheduler) startPipeline(task tasks.PipelineRun) error {
	log.Debugf(nil, "starting pipeline: %d", task.Pipeline.ID)

	ctx, cancel := context.WithCancel(context.Background())

	process := NewProcessPipeline(
		scheduler.runner,
		scheduler.runner.config.SSHKey,
		task,
		scheduler.cloud,
		log.NewChildWithPrefix(fmt.Sprintf("[pipeline:%d] ", task.Pipeline.ID)),
		ctx,
		scheduler.utilization,
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
