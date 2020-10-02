// Code generated by gonstructor -type Process -init init; DO NOT EDIT.

package job

import (
	"context"

	"github.com/reconquest/cog"
	"github.com/reconquest/snake-runner/internal/api"
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
)

func NewProcess(
	ctx context.Context,
	executor executor.Executor,
	client *api.Client,
	runnerConfig *runner.Config,
	task tasks.PipelineRun,
	configPipeline config.Pipeline,
	job snake.PipelineJob,
	log *cog.Logger,
	contextPullAuth ContextExecutorAuth,
) *Process {
	r := &Process{
		ctx:             ctx,
		executor:        executor,
		client:          client,
		runnerConfig:    runnerConfig,
		task:            task,
		configPipeline:  configPipeline,
		job:             job,
		log:             log,
		contextPullAuth: contextPullAuth,
	}

	r.init()

	return r
}