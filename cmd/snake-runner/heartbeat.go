package main

import (
	"time"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/ptr"
	"github.com/reconquest/snake-runner/internal/requests"
)

func (runner *Runner) startHeartbeats() {
	runner.workers.Add(1)
	go func() {
		defer audit.Go("heartbeats")()

		defer runner.workers.Done()
		var handshaked bool

		request := &requests.Heartbeat{
			Version: ptr.StringPtr(version),
		}

		for {
			select {
			case <-runner.context.Done():
				return
			default:
			}

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

			select {
			case <-runner.context.Done():
				return
			case <-time.After(runner.config.HeartbeatInterval):
			}
		}
	}()
}
