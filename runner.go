package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

const (
	MasterPrefixAPI = "/rest/snake/1.0"
)

var (
	FailedRegisterRepeatTimeout = time.Second * 10
)

type Runner struct {
	client *http.Client
	config *Config

	hostname string
}

func NewRunner(config *Config) *Runner {
	// TODO: move http work to a function
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	return &Runner{
		config:   config,
		client:   http.DefaultClient,
		hostname: hostname,
	}
}

func (runner *Runner) Start() {
	runner.mustRegister()
	runner.startHeartbeats()
}

func (runner *Runner) mustRegister() {
	for {
		err := runner.register()
		if err != nil {
			log.Errorf(
				err,
				"unable to register runner, will repeat after %s",
				FailedRegisterRepeatTimeout,
			)
			time.Sleep(FailedRegisterRepeatTimeout)
			continue
		}

		log.Infof(nil, "successfully registered runner")

		break
	}
}

func (runner *Runner) startHeartbeats() {
	go func() {
		for {
			err := runner.heartbeat()
			if err != nil {
				log.Errorf(err, "unable to send heartbeat")
			}

			time.Sleep(runner.config.HeartbeatInterval)
		}
	}()
}

func (runner *Runner) heartbeat() error {
	log.Infof(karma.Describe("name", runner.hostname), "sending heartbeat request")

	err := runner.request().
		POST().Path("/runner/heartbeat").
		Payload(runnerHeartbeatRequest{Name: runner.hostname}).
		Do()
	if err != nil {
		return err
	}

	return nil
}

func (runner *Runner) register() error {
	log.Infof(karma.Describe("name", runner.hostname), "sending registration request")

	err := runner.request().
		POST().Path("/runner/register").
		Payload(runnerRegisterRequest{Name: runner.hostname}).
		Do()
	if err != nil {
		return err
	}

	return nil
}

func (runner *Runner) request() *Request {
	master := strings.TrimSuffix(runner.config.MasterAddress, "/")

	return NewRequest(http.DefaultClient).
		BaseURL(master+MasterPrefixAPI).
		UserAgent("snake-runner/"+version).
		// required for authentication by snake master
		Header("X-Snake-Token", "TODO").
		// required by bitbucket itself
		Header("X-Atlassian-Token", "no-check")
}
