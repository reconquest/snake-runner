package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/api"
	"github.com/reconquest/snake-runner/internal/requests"
)

func (snake *Snake) mustRegister() string {
	for {
		token, err := snake.register()
		if err != nil {
			remote := false
			for _, reason := range karma.GetReasons(err) {
				if _, ok := reason.(api.RemoteError); ok {
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

func (snake *Snake) register() (string, error) {
	log.Infof(nil, "sending registration request")

	request := requests.NewRunnerRegister(
		snake.config.Name,
		snake.config.RegistrationToken,
	)

	response, err := snake.client.Register(*request)
	if err != nil {
		return "", err
	}

	return response.AccessToken, nil
}

func (snake *Snake) writeAccessToken(token string) error {
	dir := filepath.Dir(snake.config.AccessTokenPath)

	err := os.MkdirAll(dir, 0o664)
	if err != nil {
		return karma.Format(
			err,
			"unable to create dir for token: %s", dir,
		)
	}

	path := snake.config.AccessTokenPath
	err = ioutil.WriteFile(path, []byte(token), 0o600)
	if err != nil {
		return karma.Format(
			err,
			"unable to write data to file: %s", path,
		)
	}

	return nil
}
