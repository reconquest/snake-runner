package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

const (
	SnakeCenterAPI = "/rest/snake/1.0"
)

var (
	FailedRegisterRepeatTimeout = time.Second * 10
)

type Runner struct {
	client *http.Client
	config *Config
}

func NewRunner(config *Config) *Runner {
	return &Runner{config: config, client: http.DefaultClient}
}

func (runner *Runner) Start() {
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

func (runner *Runner) register() error {
	// TODO: move http work to a function
	hostname, err := os.Hostname()
	if err != nil {
		return karma.Format(
			err,
			"unable to obtain system hostname",
		)
	}

	log.Infof(karma.Describe("name", hostname), "sending registration request")

	buffer := bytes.NewBuffer(nil)
	err = json.NewEncoder(buffer).Encode(map[string]interface{}{
		"name": hostname,
	})
	if err != nil {
		return karma.Format(
			err,
			"unable to marshal request",
		)
	}

	method := "POST"
	url := runner.getConnectURL("/runner:register")

	context := karma.Describe("method", method).
		Describe("url", url).
		Describe("payload", buffer.String())

	request, err := http.NewRequest(method, url, buffer)
	if err != nil {
		return context.Format(
			err,
			"unable to create new request",
		)
	}

	runner.addHeaderToken(request)
	runner.addHeaderUserAgent(request)

	response, err := runner.client.Do(request)
	if err != nil {
		return context.Format(
			err,
			"unable to make http request",
		)
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return context.Format(
			err,
			"unable to read response body",
		)
	}

	log.Debugf(context.Describe("status_code", response.StatusCode), "%s", string(data))

	defer response.Body.Close()

	if response.StatusCode >= 400 {
		var errResponse responseError
		if err := json.Unmarshal(data, &errResponse); err == nil {
			return errors.New(errResponse.Error)
		} else {
			return karma.Describe("status_code", response.StatusCode).Reason(err)
		}
	}

	return nil
}

func (runner *Runner) getConnectURL(path string) string {
	address := runner.config.ConnectAddress
	if !strings.Contains(runner.config.ConnectAddress, "://") {
		address = "http://" + address
	}

	return strings.TrimSuffix(address, "/") + SnakeCenterAPI + path
}

func (runner *Runner) addHeaderUserAgent(request *http.Request) {
	request.Header.Set("User-Agent", "snake-runner/"+version)
}

func (runner *Runner) addHeaderToken(request *http.Request) {
	request.Header.Set("X-Snake-Token", "TODO")
}
