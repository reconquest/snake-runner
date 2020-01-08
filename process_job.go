package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/reconquest/cog"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/snake-runner/pkg/utils"
)

type ProcessJob struct {
	cloud *Cloud

	requester   Requester
	ctx         context.Context
	container   *Container
	cwd         string
	sshKey      string
	config      *Config
	task        TaskPipelineRun
	utilization chan *Container

	job PipelineJob
	log *cog.Logger
}

func (process *ProcessJob) sendLogs(text string) error {
	// here should be a channel with a sort of buffer

	process.log.Debugf(nil, "%s", strings.TrimSpace(text))

	err := process.requester.request().
		POST().
		Path(
			"/gate/pipelines/" + strconv.Itoa(process.task.Pipeline.ID) +
				"/jobs/" + strconv.Itoa(process.job.ID) +
				"/logs",
		).
		Payload(&RunnerLogsRequest{
			Data: text,
		}).
		Do()
	if err != nil {
		return err
	}

	return nil
}

func (process *ProcessJob) getEnv() []string {
	env := []string{}
	for key, value := range process.task.Env {
		env = append(env, key+"="+value)
	}
	return env
}

func (process *ProcessJob) execSystem(cmd ...string) error {
	err := process.sendPrompt(cmd...)
	if err != nil {
		return err
	}

	err = process.cloud.Exec(
		process.ctx,
		process.container,
		process.getEnv(),
		process.cwd,
		cmd,
		process.sendLogs,
	)
	if err != nil {
		return karma.Describe("cmd", fmt.Sprintf("%v", cmd)).
			Format(err, "command failed")
	}

	return nil
}

func (process *ProcessJob) execShell(cmd string) error {
	err := process.sendPrompt(cmd)
	if err != nil {
		return err
	}

	err = process.cloud.Exec(
		process.ctx,
		process.container,
		process.getEnv(),
		process.cwd,
		[]string{"/bin/bash", "-c", cmd},
		process.sendLogs,
	)
	if err != nil {
		return karma.Describe("cmd", fmt.Sprintf("%v", cmd)).
			Format(err, "command failed")
	}

	return nil
}

func (process *ProcessJob) sendPrompt(cmd ...string) error {
	return process.sendLogs("\n$ " + strings.Join(cmd, " ") + "\n")
}

func (process *ProcessJob) run() error {
	image := "alpine:latest"

	err := process.pullImage(image)
	if err != nil {
		return karma.Format(
			err,
			"unable to pull image: %s", image,
		)
	}

	process.container, err = process.cloud.CreateContainer(
		process.ctx,
		image,
		fmt.Sprintf(
			"pipeline-%d-job-%d-uniq-%v",
			process.task.Pipeline.ID,
			process.job.ID,
			utils.UniqHash(),
		),
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to create a container",
		)
	}

	defer func() {
		process.utilization <- process.container
	}()

	err = process.cloud.PrepareContainer(
		process.ctx,
		process.container,
		process.sshKey,
	)
	if err != nil {
		return err
	}

	err = process.prepareRepo()
	if err != nil {
		return err
	}

	err = process.readConfig()
	if err != nil {
		return err
	}

	configJob, ok := process.config.Jobs[process.job.Name]
	if !ok {
		return fmt.Errorf(
			"unable to find given job %q in %s",
			process.job.Name,
			process.task.Pipeline.Filename,
		)
	}

	for _, script := range configJob.Script {
		err = process.execShell(script)
		if err != nil {
			return err
		}
	}

	return nil
}

func (process *ProcessJob) pullImage(image string) error {
	err := process.sendPrompt("docker", "pull", image)
	if err != nil {
		return karma.Format(
			err,
			"unable to start docker pull",
		)
	}

	err = process.cloud.PullImage(process.ctx, image, process.sendLogs)
	if err != nil {
		return err
	}

	return nil
}

func (process *ProcessJob) prepareRepo() error {
	err := process.execSystem("git", "clone", process.task.CloneURL.SSH, "/home/ci/job")
	if err != nil {
		return err
	}

	process.cwd = "/home/ci/job"

	err = process.execSystem("git", "checkout", process.task.Pipeline.Commit)
	if err != nil {
		return err
	}

	return nil
}

func (process *ProcessJob) readConfig() error {
	yamlContents, err := process.readFile(process.cwd, process.task.Pipeline.Filename)
	if err != nil {
		return err
	}

	process.config, err = unmarshalConfig([]byte(yamlContents))
	if err != nil {
		return err
	}

	return nil
}

func (process *ProcessJob) readFile(cwd, path string) (string, error) {
	data := ""
	callback := func(line string) error {
		if data == "" {
			data = line
		} else {
			data = data + "\n" + line
		}

		return nil
	}

	err := process.cloud.Exec(
		process.ctx,
		process.container,
		[]string{},
		cwd,
		[]string{"cat", path},
		callback,
	)
	if err != nil {
		return "", err
	}

	return data, nil
}
