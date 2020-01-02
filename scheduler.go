package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

func (runner *Runner) startScheduler() error {
	cloud, err := NewCloud()
	if err != nil {
		return karma.Format(err, "unable to initialize container provider")
	}

	scheduler := &Scheduler{
		maxPipelines: 2,
		runner:       runner,
		cloud:        cloud,
	}

	err = cloud.Cleanup(context.Background())
	if err != nil {
		return karma.Format(err, "unable to cleanup old containers")
	}

	log.Infof(nil, "task scheduler started")

	runner.scheduler = scheduler

	go scheduler.loop()

	return nil
}

//go:generate gonstructor -type Scheduler
type Scheduler struct {
	runner       *Runner
	cloud        *Cloud
	pipelinesMap sync.Map
	pipelines    int64
	maxPipelines int64
	cancels      sync.Map
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

func (scheduler *Scheduler) serveTask(task interface{}) error {
	switch task := task.(type) {
	case TaskPipelineRun:
		atomic.AddInt64(&scheduler.pipelines, 1)

		go func() {
			defer atomic.AddInt64(&scheduler.pipelines, -1)

			err := scheduler.startPipeline(task)
			if err != nil {
				log.Errorf(err, "an error occurred during task running")
			}
		}()

	case TaskPipelineCancel:
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

func (scheduler *Scheduler) startPipeline(task TaskPipelineRun) error {
	log.Debugf(nil, "starting pipeline: %d", task.Pipeline.ID)

	ctx, cancel := context.WithCancel(context.Background())

	process := NewProcessPipeline(
		scheduler.runner,
		scheduler.runner.config.SSHKey,
		task,
		scheduler.cloud,
		log.NewChildWithPrefix(fmt.Sprintf("[pipeline:%d] ", task.Pipeline.ID)),
		ctx,
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
		} else {
			return err
		}
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
