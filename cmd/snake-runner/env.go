package main

import (
	"fmt"

	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
)

//go:generate gonstructor -type EnvBuilder
type EnvBuilder struct {
	task         tasks.PipelineRun
	pipeline     snake.Pipeline
	job          snake.PipelineJob
	config       config.Pipeline
	configJob    config.Job
	runnerConfig *RunnerConfig
	containerDir string
}

type Env struct {
	mapping map[string]string
	values  []string
}

func (env *Env) GetAll() []string {
	return env.values
}

func (env *Env) Get(key string) (string, bool) {
	value, ok := env.mapping[key]
	return value, ok
}

func (builder *EnvBuilder) Build() Env {
	mapping := builder.build()
	values := []string{}
	for key, value := range builder.build() {
		values = append(values, key+"="+value)
	}
	return Env{
		mapping: mapping,
		values:  values,
	}
}

func (builder *EnvBuilder) build() map[string]string {
	vars := map[string]string{}

	vars["CI"] = "true"

	vars["CI_PIPELINE_ID"] = fmt.Sprint(builder.pipeline.ID)
	vars["CI_JOB_ID"] = fmt.Sprint(builder.job.ID)
	vars["CI_JOB_STAGE"] = fmt.Sprint(builder.job.Stage)
	vars["CI_JOB_NAME"] = fmt.Sprint(builder.job.Name)

	if builder.pipeline.RefType == "BRANCH" {
		vars["CI_BRANCH"] = fmt.Sprint(builder.pipeline.RefDisplayId)
	} else if builder.pipeline.RefType == "TAG" {
		vars["CI_TAG"] = fmt.Sprint(builder.pipeline.RefDisplayId)
	}

	vars["CI_COMMIT_HASH"] = builder.pipeline.Commit
	if len(builder.pipeline.Commit) > 6 {
		vars["CI_COMMIT_SHORT_HASH"] = builder.pipeline.Commit[0:6]
	}

	vars["CI_PIPELINE_DIR"] = builder.containerDir

	if builder.pipeline.PullRequestID > 0 {
		vars["CI_PULL_REQUEST_ID"] = fmt.Sprint(builder.pipeline.PullRequestID)
	}

	vars["CI_PROJECT_KEY"] = builder.task.Project.Key
	vars["CI_PROJECT_NAME"] = builder.task.Project.Name
	vars["CI_PROJECT_ID"] = fmt.Sprint(builder.task.Project.ID)

	vars["CI_REPO_SLUG"] = builder.task.Repository.Slug
	vars["CI_REPO_NAME"] = builder.task.Repository.Name
	vars["CI_REPO_ID"] = fmt.Sprint(builder.task.Repository.ID)

	vars["CI_REPO_CLONE_URL_SSH"] = builder.task.CloneURL.SSH

	vars["CI_RUNNER_ID"] = fmt.Sprint(builder.pipeline.RunnerID)
	vars["CI_RUNNER_NAME"] = fmt.Sprint(builder.runnerConfig.Name)
	vars["CI_RUNNER_VERSION"] = fmt.Sprint(version)

	for key, value := range builder.task.Env {
		vars[key] = value
	}

	for key, value := range builder.config.Variables {
		vars[key] = value
	}

	for key, value := range builder.configJob.Variables {
		vars[key] = value
	}

	return vars
}
