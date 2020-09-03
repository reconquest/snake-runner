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

type Snake struct {
	config     *runner.Config
	scheduler  *Scheduler
	client     *api.Client
	context    context.Context
	cancel     context.CancelFunc
	workers    sync.WaitGroup
	terminated chan struct{}
}

func NewSnake(config *runner.Config) *Snake {
	context, cancel := context.WithCancel(context.Background())
	return &Snake{
		config:     config,
		client:     api.NewClient(config),
		context:    context,
		cancel:     cancel,
		terminated: make(chan struct{}),
	}
}

func (snake *Snake) Start() {
	accessToken := snake.config.AccessToken
	if accessToken == "" {
		// if there is no token for authentication then we need to obtain it by
		// registering the runner

		// we write empty token to token file in order to make sure that we can
		// save the token after registration otherwise users will have to
		// delete runner from CI manually and repeat registration
		err := snake.writeAccessToken("")
		if err != nil {
			log.Fatalf(
				err,
				"unable to write to access token file, make sure the process "+
					"has enough permissions",
			)
		}

		accessToken := snake.mustRegister()

		err = snake.writeAccessToken(accessToken)
		if err != nil {
			log.Fatalf(
				err,
				"unable to write to access token file, make sure the process "+
					"has enough permissions",
			)
		}

		snake.config.AccessToken = accessToken
	}

	err := snake.startScheduler()
	if err != nil {
		log.Fatalf(err, "unable to start scheduler")
	}

	snake.startHeartbeats()
}

func (snake *Snake) Shutdown() {
	snake.cancel()

	if snake.scheduler != nil {
		snake.scheduler.shutdown()
	}

	snake.workers.Wait()
}

func (snake *Snake) Terminate() {
	close(snake.terminated)
}

func (snake *Snake) Terminated() <-chan struct{} {
	return snake.terminated
}
