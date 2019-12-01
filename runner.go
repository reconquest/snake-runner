package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

const (
	MasterPrefixAPI = "/rest/snake/1.0"
	TokenHeader     = "X-Snake-Token"
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

	runner.startHeartbeats()
}

func (runner *Runner) saveToken(token string) error {
	err := os.MkdirAll(filepath.Dir(runner.config.TokenPath), 0700)
	if err != nil {
		return karma.Format(
			err,
			"unable to create dir for token",
		)
	}

	err = ioutil.WriteFile(runner.config.TokenPath, []byte(token), 0600)
	if err != nil {
		return err
	}

	return nil
}

func (runner *Runner) mustRegister() string {
	for {
		token, err := runner.register()
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

		return token
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
		Header(TokenHeader, runner.config.Token).
		Do()
	if err != nil {
		return err
	}

	return nil
}

func (runner *Runner) register() (string, error) {
	log.Infof(karma.Describe("name", runner.hostname), "sending registration request")

	var response registerResponse
	err := runner.request().
		POST().Path("/runner/register").
		Payload(registerRequest{Name: runner.hostname}).
		Response(&response).
		Do()
	if err != nil {
		return "", err
	}

	return response.AuthenticationToken, nil
}

func (runner *Runner) request() *Request {
	master := strings.TrimSuffix(runner.config.MasterAddress, "/")

	return NewRequest(http.DefaultClient).
		BaseURL(master+MasterPrefixAPI).
		UserAgent("snake-runner/"+version).
		// required by bitbucket itself
		Header("X-Atlassian-Token", "no-check")
}
