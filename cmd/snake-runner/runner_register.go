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
	"github.com/reconquest/snake-runner/internal/responses"
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

	request := requests.RunnerRegister{
		Name:      runner.name,
		PublicKey: strings.TrimSpace(string(publicKey)),
	}

	var response responses.RunnerRegister
	err = runner.request().
		POST().Path("/gate/register").
		Payload(request).
		Response(&response).
		Do()
	if err != nil {
		return "", err
	}

	return response.AuthenticationToken, nil
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
