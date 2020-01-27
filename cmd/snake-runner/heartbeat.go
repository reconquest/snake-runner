package main

import (
	"time"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/ptr"
	"github.com/reconquest/snake-runner/internal/requests"
)

func (runner *Runner) startHeartbeats() {
	go func() {
		var handshaked bool

		request := &requests.Heartbeat{
			Version: ptr.StringPtr(version),
		}

		for {
			log.Debugf(nil, "sending heartbeat request")

			err := runner.client.Heartbeat(request)
			if err != nil {
				log.Errorf(err, "unable to send heartbeat")
			} else {
				if !handshaked {
					handshaked = true
					request = &requests.Heartbeat{}
				}
			}

			time.Sleep(runner.config.HeartbeatInterval)
		}
	}()
}
