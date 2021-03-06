package env

import (
	"fmt"
	"os"

	"github.com/gonuts/go-shellquote"
	"github.com/reconquest/snake-runner/internal/builtin"
	"github.com/reconquest/snake-runner/internal/config"
	"github.com/reconquest/snake-runner/internal/consts"
	"github.com/reconquest/snake-runner/internal/runner"
	"github.com/reconquest/snake-runner/internal/snake"
	"github.com/reconquest/snake-runner/internal/tasks"
)

//go:generate gonstructor -type Builder
type Builder struct {
	task              tasks.PipelineRun
	pipeline          snake.Pipeline
	job               snake.PipelineJob
	config            config.Pipeline
	configJob         config.Job
	runnerConfig      *runner.Config
	gitDir            string
	sshSocketPath     string
	sshKnownHostsPath string
}

//go:generate gonstructor --type=Env --init init
type Env struct {
	mapping map[string]string
	values  []string `gonstructor:"-"`
}

func (env *Env) init() {
	for key, value := range env.mapping {
		env.values = append(env.values, key+"="+value)
	}
}

func (env *Env) GetAll() []string {
	return env.values
}

func (env *Env) Get(key string) (string, bool) {
	value, ok := env.mapping[key]
	return value, ok
}

func (builder *Builder) Build() *Env {
	mapping := builder.build()
	return NewEnv(mapping)
}

func (builder *Builder) build() map[string]string {
	vars := map[string]string{}

	vars["CI"] = "true"

	vars["CI_PIPELINE_ID"] = fmt.Sprint(builder.pipeline.ID)
	vars["CI_PIPELINE_DIR"] = builder.gitDir

	vars["CI_JOB_ID"] = fmt.Sprint(builder.job.ID)
	vars["CI_JOB_STAGE"] = fmt.Sprint(builder.job.Stage)
	vars["CI_JOB_NAME"] = fmt.Sprint(builder.job.Name)

	if builder.pipeline.RefType == "BRANCH" {
		vars["CI_BRANCH"] = fmt.Sprint(builder.pipeline.RefDisplayId)
	} else if builder.pipeline.RefType == "TAG" {
		vars["CI_TAG"] = fmt.Sprint(builder.pipeline.RefDisplayId)
	}

	vars["CI_REF"] = builder.pipeline.RefDisplayId
	vars["CI_REF_TYPE"] = builder.pipeline.RefType

	vars["CI_COMMIT_HASH"] = builder.pipeline.Commit
	if len(builder.pipeline.Commit) > 6 {
		vars["CI_COMMIT_SHORT_HASH"] = builder.pipeline.Commit[0:6]
	}

	vars["CI_FROM_COMMIT_HASH"] = builder.pipeline.FromCommit
	if len(builder.pipeline.FromCommit) > 6 {
		vars["CI_FROM_COMMIT_SHORT_HASH"] = builder.pipeline.FromCommit[0:6]
	}

	if builder.pipeline.PullRequestID > 0 {
		vars["CI_PULL_REQUEST_ID"] = fmt.Sprint(builder.pipeline.PullRequestID)
	}

	if builder.task.PullRequest != nil {
		pr := builder.task.PullRequest
		vars["CI_PULL_REQUEST_ID"] = fmt.Sprint(pr.ID)
		vars["CI_PULL_REQUEST_TITLE"] = pr.Title
		vars["CI_PULL_REQUEST_STATE"] = pr.State
		vars["CI_PULL_REQUEST_CROSS_REPO"] = fmt.Sprint(pr.IsCrossRepo)

		vars["CI_PULL_REQUEST_FROM_HASH"] = pr.FromRef.Hash
		vars["CI_PULL_REQUEST_FROM_REF"] = pr.FromRef.Ref
		vars["CI_PULL_REQUEST_FROM_FORK"] = fmt.Sprint(pr.FromRef.IsFork)

		vars["CI_PULL_REQUEST_TO_HASH"] = pr.ToRef.Hash
		vars["CI_PULL_REQUEST_TO_REF"] = pr.ToRef.Ref
		vars["CI_PULL_REQUEST_TO_FORK"] = fmt.Sprint(pr.ToRef.IsFork)
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
	vars["CI_RUNNER_VERSION"] = fmt.Sprint(builtin.Version)

	// special case: providing SSH_AUTH_SOCK — socket to ssh-agent that is
	// running in sidecar
	vars[consts.SSH_AUTH_SOCK_VAR] = builder.sshSocketPath

	expand := func(key string) string {
		result, ok := vars[key]
		if !ok {
			return os.Getenv(key)
		}

		return result
	}

	for key, value := range builder.task.Env {
		vars[key] = os.Expand(value, expand)
	}

	if builder.config.Variables != nil {
		for _, pair := range builder.config.Variables.Pairs() {
			vars[pair.Key] = os.Expand(pair.Value, expand)
		}
	}

	if builder.configJob.Variables != nil {
		for _, pair := range builder.configJob.Variables.Pairs() {
			vars[pair.Key] = os.Expand(pair.Value, expand)
		}
	}

	// special case: providing GIT_SSH_COMMAND with a global known hosts file
	// specified
	vars[consts.GIT_SSH_COMMAND_VAR] = shellquote.Join(
		"ssh", "-o"+consts.SSH_OPTION_GLOBAL_HOSTS_FILE+"="+builder.sshKnownHostsPath,
	)

	// providing path to generated known_hosts so users can use it with ssh
	vars["CI_SSH_KNOWN_HOSTS_FILE"] = builder.sshKnownHostsPath

	return vars
}
