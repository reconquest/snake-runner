package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/api"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/executor/docker"
	"github.com/reconquest/snake-runner/internal/executor/shell"
	"github.com/reconquest/snake-runner/internal/pipeline"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/safemap"
	"github.com/reconquest/snake-runner/internal/signal"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/reconquest/snake-runner/internal/utils"
)

type Scheduler struct {
	client         *api.Client
	executor       executor.Executor
	pipelinesMap   safemap.IntToAny
	pipelines      int64
	pipelinesGroup sync.WaitGroup
	cancels        safemap.IntToContextCancelFunc
	runnerConfig   *runner.Config

	sshKeyFactory *sshkey.Factory
	sshKey        *sshkey.Key

	context  context.Context
	cancel   func()
	routines sync.WaitGroup
	loopWork sync.WaitGroup

	terminator utils.Terminator
	alive      bool
}

func (snake *Snake) startScheduler() error {
	var execer executor.Executor
	var err error
	switch snake.config.Mode {
	case runner.RUNNER_MODE_DOCKER:
		log.Infof(nil, "initializing docker provider")

		execer, err = docker.NewDocker(
			snake.config.Docker.Network,
			snake.config.Docker.Volumes,
		)
		if err != nil {
			return karma.Format(err, "unable to initialize new container provider")
		}

	case runner.RUNNER_MODE_SHELL:
		log.Infof(nil, "initializing shell provider")

		execer, err = shell.NewShell()
		if err != nil {
			return karma.Format(
				err,
				"unable to initialize new shell provider",
			)
		}

	default:
		return fmt.Errorf(
			"unexpected runner mode: %s", snake.config.Mode,
		)
	}

	ctx, cancel := context.WithCancel(context.Background())

	scheduler := &Scheduler{
		client:       snake.client,
		executor:     execer,
		runnerConfig: snake.config,
		sshKeyFactory: sshkey.NewFactory(
			ctx,
			int(snake.config.MaxParallelPipelines),
			sshkey.DEFAULT_BLOCK_SIZE,
		),
		pipelinesMap: safemap.NewIntToAny(),
		cancels:      safemap.NewIntToContextCancelFunc(),
		context:      ctx,
		cancel:       cancel,
		terminator:   snake,
	}

	err = execer.Cleanup()
	if err != nil {
		return karma.Format(err, "unable to cleanup old resources")
	}

	log.Infof(nil, "task scheduler started")

	snake.scheduler = scheduler

	snake.scheduler.start()

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

		scheduler.alive = true

		if wait {
			log.Tracef(
				nil,
				"sleeping %v",
				scheduler.runnerConfig.SchedulerInterval,
			)

			select {
			case <-scheduler.context.Done():
				return
			case <-time.After(scheduler.runnerConfig.SchedulerInterval):
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
		pipelines < scheduler.runnerConfig.MaxParallelPipelines,
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

func (scheduler *Scheduler) serveTask(task interface{}, sshKey sshkey.Key) error {
	switch task := task.(type) {
	case *tasks.PipelineRun:
		scheduler.startPipeline(*task, sshKey)

	case *tasks.PipelineCancel:
		for _, id := range task.Pipelines {
			scheduler.cancelPipeline(id)
		}

	case *tasks.RunnerTerminate:
		log.Infof(
			karma.Describe("reason", task.Reason),
			"terminate: runner received termination signal",
		)
		if !scheduler.alive {
			log.Warningf(nil, "terminate: restart of deleted runner is detected")
			log.Warningf(nil, "terminate: suspending runner to prevent restart loop")
			scheduler.cancel()
		} else {
			scheduler.terminator.Terminate()
			<-scheduler.context.Done()
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

	process := pipeline.NewProcess(
		scheduler.context,
		ctx,
		scheduler.client,
		scheduler.runnerConfig,
		task,
		scheduler.executor,
		log.NewChildWithPrefix(fmt.Sprintf("[pipeline:%d]", task.Pipeline.ID)),
		sshKey,
		signal.NewCondition(),
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

		err := process.Run()
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

	scheduler.routines.Wait()

	log.Warningf(nil, "shutdown: scheduler gracefully terminated")
}
