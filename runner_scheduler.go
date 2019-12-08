package main

import (
	"fmt"
	"os"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

func (runner *Runner) startScheduler() {
	scheduler := &RunnerScheduler{
		spots:  make(chan struct{}, 2),
		runner: runner,
	}
	go scheduler.loop()

	log.Infof(nil, "scheduler started")
}

type RunnerScheduler struct {
	spots  chan struct{}
	runner *Runner
}

func (scheduler *RunnerScheduler) lockSpot() {
	// the main idea here of these three components:
	// chan & writer & reader is that we 'hold' a spot for task by writing an
	// item into channel which will be blocked if the channel is full already
	// and will wait for an unlock from another routine
	scheduler.spots <- struct{}{}
}

func (scheduler *RunnerScheduler) unlockSpot() {
	<-scheduler.spots
}

func (scheduler *RunnerScheduler) loop() {
	for {
		scheduler.lockSpot()

		go func() {
			defer scheduler.unlockSpot()

			err := scheduler.schedule()
			if err != nil {
				log.Errorf(err, "an error occurred during task scheduling")
			}
		}()

		time.Sleep(scheduler.runner.config.SchedulerInterval)
	}
}

func (scheduler *RunnerScheduler) schedule() error {
	log.Debugf(nil, "retrieving a task")

	task, err := scheduler.runner.getTask()
	if err != nil {
		return karma.Format(
			err,
			"unable to retrieve task",
		)
	}

	fmt.Fprintf(os.Stderr, "XXXXXX runner_scheduler.go:64 task: %#v\n", task)
	time.Sleep(time.Second * 10)

	return nil
}
