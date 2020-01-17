package main

import (
	"time"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/requests"
)

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
	log.Debugf(nil, "sending heartbeat request")

	err := runner.request().
		POST().Path("/gate/heartbeat").
		Payload(requests.Heartbeat{}).
		Do()
	if err != nil {
		return err
	}

	return nil
}
