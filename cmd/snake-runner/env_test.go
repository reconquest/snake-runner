package main

import (
	"testing"

	"github.com/reconquest/snake-runner/internal/responses"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
	"github.com/stretchr/testify/assert"
)

func TestEnvBuilder(t *testing.T) {
	test := assert.New(t)

	pipeline := snake.Pipeline{
		ID:           123,
		RefType:      "BRANCH",
		RefDisplayId: "branch1",
		Commit:       "1234567890",
		RunnerID:     80,
	}
	job := snake.PipelineJob{
		ID:    321,
		Stage: "deploy",
		Name:  "docker deploy",
	}
	config := &RunnerConfig{
		Name: "gotest",
	}
	task := tasks.PipelineRun{
		Pipeline: pipeline,
		Env:      map[string]string{"user_a": "user_a_value"},
		Project: responses.Project{
			Key:  "proj1",
			Name: "the proj1",
			ID:   11,
		},
		Repository: responses.Repository{
			Slug: "repo1",
			Name: "the repo1",
			ID:   111,
		},
	}
	task.CloneURL.SSH = "cloneurl"

	builder := NewEnvBuilder(task, pipeline, job, config, "/dir")
	test.EqualValues([]string{
		"user_a=user_a_value",
		"SNAKE_CI=true",
		"CI_PIPELINE_ID=123",
		"CI_JOB_ID=321",
		"CI_JOB_STAGE=deploy",
		"CI_JOB_NAME=docker deploy",
		"CI_BRANCH=branch1",
		"CI_COMMIT_HASH=1234567890",
		"CI_COMMIT_SHORT_HASH=123456",
		"CI_PIPELINE_DIR=/dir",
		"CI_PROJECT_KEY=proj1",
		"CI_PROJECT_NAME=the proj1",
		"CI_PROJECT_ID=11",
		"CI_REPO_SLUG=repo1",
		"CI_REPO_NAME=the repo1",
		"CI_REPO_ID=111",
		"CI_REPO_CLONE_URL_SSH=cloneurl",
		"CI_RUNNER_ID=80",
		"CI_RUNNER_NAME=gotest",
		"CI_RUNNER_VERSION=" + version,
	}, builder.build())

	builder.pipeline.RefType = "TAG"
	test.EqualValues([]string{
		"user_a=user_a_value",
		"SNAKE_CI=true",
		"CI_PIPELINE_ID=123",
		"CI_JOB_ID=321",
		"CI_JOB_STAGE=deploy",
		"CI_JOB_NAME=docker deploy",
		"CI_TAG=branch1",
		"CI_COMMIT_HASH=1234567890",
		"CI_COMMIT_SHORT_HASH=123456",
		"CI_PIPELINE_DIR=/dir",
		"CI_PROJECT_KEY=proj1",
		"CI_PROJECT_NAME=the proj1",
		"CI_PROJECT_ID=11",
		"CI_REPO_SLUG=repo1",
		"CI_REPO_NAME=the repo1",
		"CI_REPO_ID=111",
		"CI_REPO_CLONE_URL_SSH=cloneurl",
		"CI_RUNNER_ID=80",
		"CI_RUNNER_NAME=gotest",
		"CI_RUNNER_VERSION=" + version,
	}, builder.build())

	builder.pipeline.RefType = ""
	test.EqualValues([]string{
		"user_a=user_a_value",
		"SNAKE_CI=true",
		"CI_PIPELINE_ID=123",
		"CI_JOB_ID=321",
		"CI_JOB_STAGE=deploy",
		"CI_JOB_NAME=docker deploy",
		"CI_COMMIT_HASH=1234567890",
		"CI_COMMIT_SHORT_HASH=123456",
		"CI_PIPELINE_DIR=/dir",
		"CI_PROJECT_KEY=proj1",
		"CI_PROJECT_NAME=the proj1",
		"CI_PROJECT_ID=11",
		"CI_REPO_SLUG=repo1",
		"CI_REPO_NAME=the repo1",
		"CI_REPO_ID=111",
		"CI_REPO_CLONE_URL_SSH=cloneurl",
		"CI_RUNNER_ID=80",
		"CI_RUNNER_NAME=gotest",
		"CI_RUNNER_VERSION=" + version,
	}, builder.build())
}
