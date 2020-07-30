package main

import (
	"context"
	"sync"
	"time"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/api"
	"github.com/reconquest/snake-runner/internal/runner"
)

var FailedRegisterRepeatTimeout = time.Second * 10

type Runner struct {
	config     *runner.Config
	scheduler  *Scheduler
	client     *api.Client
	context    context.Context
	cancel     context.CancelFunc
	workers    sync.WaitGroup
	terminated chan struct{}
}

func NewRunner(config *runner.Config) *Runner {
	context, cancel := context.WithCancel(context.Background())
	return &Runner{
		config:     config,
		client:     api.NewClient(config),
		context:    context,
		cancel:     cancel,
		terminated: make(chan struct{}),
	}
}

func (runner *Runner) Start() {
	accessToken := runner.config.AccessToken
	if accessToken == "" {
		// if there is no token for authentication then we need to obtain it by
		// registering the runner

		// we write empty token to token file in order to make sure that we can
		// save the token after registration otherwise users will have to
		// delete runner from CI manually and repeat registration
		err := runner.writeAccessToken("")
		if err != nil {
			log.Fatalf(
				err,
				"unable to write to access token file, make sure the process "+
					"has enough permissions",
			)
		}

		accessToken := runner.mustRegister()

		err = runner.writeAccessToken(accessToken)
		if err != nil {
			log.Fatalf(
				err,
				"unable to write to access token file, make sure the process "+
					"has enough permissions",
			)
		}

		runner.config.AccessToken = accessToken
	}

	err := runner.startScheduler()
	if err != nil {
		log.Fatalf(err, "unable to start scheduler")
	}

	runner.startHeartbeats()
}

func (runner *Runner) Shutdown() {
	runner.cancel()

	if runner.scheduler != nil {
		runner.scheduler.shutdown()
	}

	runner.workers.Wait()
}

func (runner *Runner) Terminate() {
	close(runner.terminated)
}

func (runner *Runner) Terminated() <-chan struct{} {
	return runner.terminated
}
