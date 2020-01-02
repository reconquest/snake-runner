package main

import (
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

	err = cloud.Cleanup()
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
}

func (scheduler *Scheduler) loop() {
	for {
		pipelines := atomic.LoadInt64(&scheduler.pipelines)

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
	case TaskPipeline:
		atomic.AddInt64(&scheduler.pipelines, 1)

		go func() {
			defer atomic.AddInt64(&scheduler.pipelines, -1)

			err := scheduler.startPipeline(task)
			if err != nil {
				log.Errorf(err, "an error occurred during task running")
			}
		}()

	default:
		log.Errorf(nil, "unexpected type of task %#v: %T", task, task)
	}

	return nil
}

func (scheduler *Scheduler) startPipeline(task TaskPipeline) error {
	log.Debugf(nil, "starting pipeline: %d", task.Pipeline.ID)

	process := NewProcessPipeline(
		scheduler.runner,
		scheduler.runner.config.SSHKey,
		task,
		scheduler.cloud,
		log.NewChildWithPrefix(fmt.Sprintf("[pipeline:%d] ", task.Pipeline.ID)),
	)

	scheduler.pipelinesMap.Store(task.Pipeline.ID, struct{}{})
	defer scheduler.pipelinesMap.Delete(task.Pipeline.ID)

	err := process.run()
	if err != nil {
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
