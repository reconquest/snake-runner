// Code generated by gonstructor -type EnvBuilder; DO NOT EDIT.

package main

import (
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
)

func NewEnvBuilder(task tasks.PipelineRun, pipeline snake.Pipeline, job snake.PipelineJob, config config.Pipeline, configJob config.Job, runnerConfig *RunnerConfig, gitDir string, sshDir string) *EnvBuilder {
	return &EnvBuilder{task: task, pipeline: pipeline, job: job, config: config, configJob: configJob, runnerConfig: runnerConfig, gitDir: gitDir, sshDir: sshDir}
}
