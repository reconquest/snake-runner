package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/requests"
)

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

func (runner *Runner) register() (string, error) {
	log.Infof(nil, "sending registration request")

	publicKey, err := ioutil.ReadFile(runner.config.SSHKey + ".pub")
	if err != nil {
		return "", karma.Format(
			err,
			"unable to read ssh key public part",
		)
	}

	request := requests.NewRunnerRegister(
		runner.config.Name,
		runner.config.RegistrationToken,
		strings.TrimSpace(string(publicKey)),
	)

	response, err := runner.client.Register(*request)
	if err != nil {
		return "", err
	}

	return response.AccessToken, nil
}

func (runner *Runner) writeAccessToken(token string) error {
	dir := filepath.Dir(runner.config.AccessTokenPath)
	err := os.MkdirAll(dir, 0600)
	if err != nil {
		return karma.Format(
			err,
			"unable to create dir for token: %s", dir,
		)
	}

	path := runner.config.AccessTokenPath
	err = ioutil.WriteFile(path, []byte(token), 0600)
	if err != nil {
		return karma.Format(
			err,
			"unable to write data to file: %s", path,
		)
	}

	return nil
}
