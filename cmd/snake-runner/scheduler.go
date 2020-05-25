package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/cloud"
	"github.com/reconquest/snake-runner/internal/safemap"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/tasks"
)

type Scheduler struct {
	client         *Client
	cloud          *cloud.Cloud
	pipelinesMap   safemap.IntToAny
	pipelines      int64
	pipelinesGroup sync.WaitGroup
	cancels        safemap.IntToContextCancelFunc
	utilization    chan *cloud.Container
	config         *RunnerConfig

	sshKeyFactory *sshkey.Factory
	sshKey        *sshkey.Key

	context  context.Context
	cancel   func()
	routines sync.WaitGroup
	loopWork sync.WaitGroup
}

func (runner *Runner) startScheduler() error {
	docker, err := cloud.NewDocker(
		runner.config.Docker.Network,
		runner.config.Docker.Volumes,
	)
	if err != nil {
		return karma.Format(err, "unable to initialize container provider")
	}

	ctx, cancel := context.WithCancel(context.Background())

	scheduler := &Scheduler{
		client:      runner.client,
		cloud:       docker,
		utilization: make(chan *cloud.Container, runner.config.MaxParallelPipelines*2),
		config:      runner.config,
		sshKeyFactory: sshkey.NewFactory(
			ctx,
			int(runner.config.MaxParallelPipelines),
			sshkey.DefaultBlockSize,
		),
		pipelinesMap: safemap.NewIntToAny(),
		cancels:      safemap.NewIntToContextCancelFunc(),
		context:      ctx,
		cancel:       cancel,
	}

	err = docker.Cleanup(context.Background())
	if err != nil {
		return karma.Format(err, "unable to cleanup old containers")
	}

	log.Infof(nil, "task scheduler started")

	runner.scheduler = scheduler

	runner.scheduler.start()

	return nil
}

func (scheduler *Scheduler) start() {
	scheduler.routines.Add(3)
	go func() {
		defer audit.Go("scheduler", "ssh key factory")()
		defer scheduler.routines.Done()
		scheduler.sshKeyFactory.Run()
	}()
	go func() {
		defer audit.Go("scheduler", "loop")()
		defer scheduler.routines.Done()
		scheduler.loop()
	}()
	go func() {
		defer audit.Go("scheduler", "utilizer")()
		defer scheduler.routines.Done()
		scheduler.utilize()
	}()
}

func (scheduler *Scheduler) loop() {
	scheduler.loopWork.Add(1)
	defer scheduler.loopWork.Done()

	for {
		select {
		case <-scheduler.context.Done():
			return
		default:
		}

		wait, err := scheduler.getAndServe()
		if err != nil {
			log.Error(err)
		}

		if wait {
			log.Tracef(nil, "sleeping %v", scheduler.config.SchedulerInterval)
			select {
			case <-scheduler.context.Done():
				return
			case <-time.After(scheduler.config.SchedulerInterval):
			}
		}
	}
}

func (scheduler *Scheduler) getAndServe() (bool, error) {
	var err error

	if scheduler.sshKey == nil {
		select {
		case scheduler.sshKey = <-scheduler.sshKeyFactory.Get():
			//
		case <-scheduler.context.Done():
			return false, nil
		}
	}

	pipelines := atomic.LoadInt64(&scheduler.pipelines)

	log.Debugf(nil, "retrieving task [running pipelines: %d]", pipelines)

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
		scheduler.startPipeline(task, sshKey)

	case tasks.PipelineCancel:
		for _, id := range task.Pipelines {
			scheduler.cancelPipeline(id)
		}

	default:
		log.Errorf(nil, "unexpected type of task %#v: %T", task, task)
	}

	return nil
}

func (scheduler *Scheduler) cancelPipeline(id int) {
	cancel, ok := scheduler.cancels.Load(id)
	if !ok {
		log.Warningf(
			nil,
			"unable to cancel pipeline %d, its context already gone",
			id,
		)
	} else {
		log.Infof(nil, "task: canceling pipeline: %d", id)
		cancel()

		scheduler.cancels.Delete(id)
		scheduler.pipelinesMap.Delete(id)
	}
}

func (scheduler *Scheduler) startPipeline(
	task tasks.PipelineRun,
	sshKey sshkey.Key,
) {
	log.Debugf(nil, "starting pipeline: %d", task.Pipeline.ID)

	ctx, cancel := context.WithCancel(context.Background())

	process := NewProcessPipeline(
		scheduler.context,
		ctx,
		scheduler.client,
		scheduler.config,
		task,
		scheduler.cloud,
		log.NewChildWithPrefix(fmt.Sprintf("[pipeline:%d]", task.Pipeline.ID)),
		scheduler.utilization,
		sshKey,
	)

	scheduler.pipelinesMap.Store(task.Pipeline.ID, struct{}{})
	scheduler.cancels.Store(task.Pipeline.ID, cancel)
	atomic.AddInt64(&scheduler.pipelines, 1)
	scheduler.pipelinesGroup.Add(1)

	go func() {
		defer audit.Go("pipeline", task.Pipeline.ID)()

		defer scheduler.pipelinesMap.Delete(task.Pipeline.ID)
		defer scheduler.cancels.Delete(task.Pipeline.ID)
		defer atomic.AddInt64(&scheduler.pipelines, -1)
		defer scheduler.pipelinesGroup.Done()

		err := process.run()
		if err != nil {
			if karma.Contains(err, context.Canceled) {
				log.Infof(nil, "pipeline %d finished due to cancel", task.Pipeline.ID)
				return
			}

			log.Debug(
				karma.Format(
					err,
					"pipeline=%d an error occurred during task running",
					task.Pipeline.ID,
				),
			)
		}
	}()
}

func (scheduler *Scheduler) getPipelines() []int {
	result := []int{}

	scheduler.pipelinesMap.Range(func(id int, _ safemap.Any) bool {
		result = append(result, id)
		return true
	})

	return result
}

func (scheduler *Scheduler) shutdown() {
	log.Warningf(nil, "shutdown: terminating heartbeat and task routines")

	scheduler.cancel()
	scheduler.loopWork.Wait()

	ids := []int{}
	scheduler.pipelinesMap.Range(func(id int, _ safemap.Any) bool {
		ids = append(ids, id)
		return true
	})

	for _, id := range ids {
		log.Warningf(nil, "shutdown: canceling pipeline: %v", id)
		scheduler.cancelPipeline(id)
	}

	go func() {
		defer audit.Go("shutdown", "waiter")()

		for {
			pipelines := atomic.LoadInt64(&scheduler.pipelines)

			log.Warningf(
				nil,
				"shutdown: waiting for pipelines to be terminated: %d",
				pipelines,
			)

			if pipelines == 0 {
				break
			}

			time.Sleep(time.Second)
		}
	}()
	scheduler.pipelinesGroup.Wait()

	log.Warningf(nil, "shutdown: waiting for all containers to be terminated")

	close(scheduler.utilization)
	scheduler.routines.Wait()

	log.Warningf(nil, "shutdown: scheduler gracefully terminated")
}
