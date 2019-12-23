package main

import (
	"fmt"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

func (runner *Runner) startScheduler() {
	cloud, err := NewCloud()
	if err != nil {
		log.Fatalf(err, "unable to initialize container provider")
	}

	scheduler := &Scheduler{
		spots:  make(chan struct{}, 5),
		runner: runner,
		cloud:  cloud,
	}

	err = cloud.Cleanup()
	if err != nil {
		log.Fatal(err)
	}

	go scheduler.loop()

	log.Infof(nil, "scheduler started")
}

//go:generate gonstructor -type Scheduler
type Scheduler struct {
	spots  chan struct{}
	runner *Runner
	cloud  *Cloud
}

func (scheduler *Scheduler) lockSpot() {
	// the main idea here of these three components:
	// chan & writer & reader is that we 'hold' a spot for task by writing an
	// item into channel which will be blocked if the channel is full already
	// and will wait for an unlock from another routine
	scheduler.spots <- struct{}{}
}

func (scheduler *Scheduler) unlockSpot() {
	<-scheduler.spots
}

func (scheduler *Scheduler) loop() {
	for {
		scheduler.lockSpot()

		go func() {
			defer scheduler.unlockSpot()

			err := scheduler.startProcess()
			if err != nil {
				log.Errorf(err, "an error occurred during task running")
			}
		}()

		time.Sleep(scheduler.runner.config.SchedulerInterval)
	}
}

func (scheduler *Scheduler) startProcess() error {
	log.Debugf(nil, "retrieving a task")

	task, err := scheduler.runner.getTask()
	if err != nil {
		return karma.Format(
			err,
			"unable to retrieve task",
		)
	}

	// no task @ no problem
	if task.Pipeline.ID == 0 {
		log.Debugf(nil, "no tasks received")
		return nil
	}

	process := NewProcessPipeline(
		scheduler.runner,
		scheduler.runner.config.SSHKey,
		task,
		scheduler.cloud,
		log.NewChildWithPrefix(fmt.Sprintf("[pipeline:%d] ", task.Pipeline.ID)),
	)

	err = process.run()
	if err != nil {
		return err
	}

	return nil
}
