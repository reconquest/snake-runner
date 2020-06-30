package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/requests"
)

func (runner *Runner) mustRegister() string {
	for {
		token, err := runner.register()
		if err != nil {
			remote := false
			for _, reason := range karma.GetReasons(err) {
				if _, ok := reason.(remoteError); ok {
					remote = true
					break
				}
			}
			if remote {
				log.Errorf(err, "make sure you have passed correct registration token")
			} else {
				log.Errorf(
					err,
					"unable to register runner, will repeat after %s",
					FailedRegisterRepeatTimeout,
				)
			}

			time.Sleep(FailedRegisterRepeatTimeout)
			continue
		}

		log.Infof(nil, "successfully registered runner")

		return token
	}
}

func (runner *Runner) register() (string, error) {
	log.Infof(nil, "sending registration request")

	request := requests.NewRunnerRegister(
		runner.config.Name,
		runner.config.RegistrationToken,
	)

	response, err := runner.client.Register(*request)
	if err != nil {
		return "", err
	}

	return response.AccessToken, nil
}

func (runner *Runner) writeAccessToken(token string) error {
	dir := filepath.Dir(runner.config.AccessTokenPath)
	err := os.MkdirAll(dir, 0o664)
	if err != nil {
		return karma.Format(
			err,
			"unable to create dir for token: %s", dir,
		)
	}

	path := runner.config.AccessTokenPath
	err = ioutil.WriteFile(path, []byte(token), 0o600)
	if err != nil {
		return karma.Format(
			err,
			"unable to write data to file: %s", path,
		)
	}

	return nil
}
