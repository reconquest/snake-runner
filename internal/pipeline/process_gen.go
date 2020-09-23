// Code generated by gonstructor -type Process; DO NOT EDIT.

package pipeline

import (
	"context"

	"github.com/reconquest/cog"
	"github.com/reconquest/snake-runner/internal/api"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/signal"
	"github.com/reconquest/snake-runner/internal/sshkey"
	"github.com/reconquest/snake-runner/internal/tasks"
)

func NewProcess(
	parentCtx context.Context,
	ctx context.Context,
	client *api.Client,
	runnerConfig *runner.Config,
	task tasks.PipelineRun,
	executor executor.Executor,
	log *cog.Logger,
	sshKey sshkey.Key,
	configCond signal.Condition,
) *Process {
	return &Process{
		parentCtx:    parentCtx,
		ctx:          ctx,
		client:       client,
		runnerConfig: runnerConfig,
		task:         task,
		executor:     executor,
		log:          log,
		sshKey:       sshKey,
		configCond:   configCond,
	}
}
