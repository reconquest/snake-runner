package main

import (
	"fmt"

	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
)

//go:generate gonstructor -type EnvBuilder
type EnvBuilder struct {
	task         tasks.PipelineRun
	pipeline     snake.Pipeline
	job          snake.PipelineJob
	config       *RunnerConfig
	containerDir string
}

func (builder *EnvBuilder) build() []string {
	vars := []string{}
	add := func(key, value string) {
		vars = append(vars, key+"="+value)
	}

	for key, value := range builder.task.Env {
		add(key, value)
	}

	add("SNAKE_CI", "true")

	add("CI_PIPELINE_ID", fmt.Sprint(builder.pipeline.ID))
	add("CI_JOB_ID", fmt.Sprint(builder.job.ID))
	add("CI_JOB_STAGE", fmt.Sprint(builder.job.Stage))
	add("CI_JOB_NAME", fmt.Sprint(builder.job.Name))

	if builder.pipeline.RefType == "BRANCH" {
		add("CI_BRANCH", fmt.Sprint(builder.pipeline.RefDisplayId))
	} else if builder.pipeline.RefType == "TAG" {
		add("CI_TAG", fmt.Sprint(builder.pipeline.RefDisplayId))
	}

	add("CI_COMMIT_HASH", builder.pipeline.Commit)
	if len(builder.pipeline.Commit) > 6 {
		add("CI_COMMIT_SHORT_HASH", builder.pipeline.Commit[0:6])
	}

	add("CI_PIPELINE_DIR", builder.containerDir)

	if builder.pipeline.PullRequestID > 0 {
		add("CI_PULL_REQUEST_ID", fmt.Sprint(builder.pipeline.PullRequestID))
	}

	add("CI_PROJECT_KEY", builder.task.Project.Key)
	add("CI_PROJECT_NAME", builder.task.Project.Name)
	add("CI_PROJECT_ID", fmt.Sprint(builder.task.Project.ID))

	add("CI_REPO_SLUG", builder.task.Repository.Slug)
	add("CI_REPO_NAME", builder.task.Repository.Name)
	add("CI_REPO_ID", fmt.Sprint(builder.task.Repository.ID))

	add("CI_REPO_CLONE_URL_SSH", builder.task.CloneURL.SSH)

	add("CI_RUNNER_ID", fmt.Sprint(builder.pipeline.RunnerID))
	add("CI_RUNNER_NAME", fmt.Sprint(builder.config.Name))
	add("CI_RUNNER_VERSION", fmt.Sprint(version))

	return vars
}
