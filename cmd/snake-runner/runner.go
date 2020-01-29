package main

import (
	"context"
	"sync"
	"time"

	"github.com/reconquest/pkg/log"
)

const (
	MasterPrefixAPI   = "/rest/snake-ci/1.0"
	AccessTokenHeader = "X-Snake-Runner-Access-Token"
	NameHeader        = "X-Snake-Runner-Name"
)

var FailedRegisterRepeatTimeout = time.Second * 10

type Runner struct {
	config    *RunnerConfig
	scheduler *Scheduler
	client    *Client
	context   context.Context
	cancel    context.CancelFunc
	workers   sync.WaitGroup
}

func NewRunner(config *RunnerConfig) *Runner {
	context, cancel := context.WithCancel(context.Background())
	return &Runner{
		config:  config,
		client:  NewClient(config),
		context: context,
		cancel:  cancel,
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
