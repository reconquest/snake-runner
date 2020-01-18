package main

import (
	"time"

	"github.com/reconquest/pkg/log"
)

const (
	MasterPrefixAPI = "/rest/snake/1.0"
	TokenHeader     = "X-Snake-Runner-Token"
	NameHeader      = "X-Snake-Runner-Name"
)

var FailedRegisterRepeatTimeout = time.Second * 10

type Runner struct {
	config    *RunnerConfig
	scheduler *Scheduler
	client    *Client
}

func NewRunner(config *RunnerConfig) *Runner {
	return &Runner{
		config: config,
		client: NewClient(config),
	}
}

func (runner *Runner) Start() {
	token := runner.config.Token
	if token == "" {
		// if there is no token for authentication then we need to obtain it by
		// registering the runner

		// we write empty token to token file in order to make sure that we can
		// save the token after registration otherwise users will have to
		// delete runner from CI manually and repeat registration
		err := runner.saveToken("")
		if err != nil {
			log.Fatalf(
				err,
				"unable to write to token file, make sure process "+
					"has enough permissions",
			)
		}

		token := runner.mustRegister()

		err = runner.saveToken(token)
		if err != nil {
			log.Fatalf(err, "unable to save token to %s", runner.config.TokenPath)
		}

		runner.config.Token = token
	}

	err := runner.startScheduler()
	if err != nil {
		log.Fatalf(err, "unable to start scheduler")
	}

	runner.startHeartbeats()
}
