package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/kovetskiy/ko"
	"github.com/reconquest/executil-go"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

type RunnerConfig struct {
	ListenAddress string `yaml:"listen_address"  env:"SNAKE_LISTEN_ADDRESS" required:"true" default:":8585" `
	MasterAddress string `yaml:"master_address" env:"SNAKE_MASTER_ADDRESS" required:"true"`

	Log struct {
		Debug bool `yaml:"debug" env:"SNAKE_LOG_DEBUG"`
		Trace bool `yaml:"trace" env:"SNAKE_LOG_TRACE"`
	}

	Name string `yaml:"name" env:"SNAKE_NAME"`

	Token     string `yaml:"token" env:"SNAKE_TOKEN"`
	TokenPath string `yaml:"token_path" env:"SNAKE_TOKEN_PATH" default:"/etc/snake-runner/token"`

	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" default:"30s"`
	SchedulerInterval time.Duration `yaml:"scheduler_interval" default:"5s"`

	Virtualization       string `yaml:"virtualization" default:"docker" required:"true"`
	MaxParallelPipelines int64  `yaml:"max_parallel_pipelines" default:"0" required:"true"`

	SSHKey string `yaml:"ssh_key" env:"SNAKE_SSH_KEY_PATH" default:"/etc/snake-runner/id_rsa" required:"true"`
}

func LoadRunnerConfig(path string) (*RunnerConfig, error) {
	log.Infof(karma.Describe("path", path), "loading configuration")

	var config RunnerConfig
	err := ko.Load(path, &config, yaml.Unmarshal)
	if err != nil {
		return nil, err
	}

	if config.TokenPath != "" && config.Token == "" {
		tokenData, err := ioutil.ReadFile(config.TokenPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, karma.Format(
				err,
				"unable to read specified token file: %s", config.TokenPath,
			)
		}

		config.Token = strings.TrimSpace(string(tokenData))
	}

	if config.Virtualization == "none" {
		log.Warningf(nil, "No virtualization is used, all commands will be "+
			"executed on the local host with current permissions")
	}

	if !isFileExists(config.SSHKey) && !isFileExists(config.SSHKey+".pub") {
		log.Warningf(nil, "ssh key not found at %s, generating it", config.SSHKey)

		_, _, err := executil.Run(
			exec.Command("ssh-keygen", "-t", "rsa", "-C", "snake-runner", "-f", config.SSHKey),
		)
		if err != nil {
			return nil, karma.Format(
				err,
				"unable to generate ssh-key",
			)
		}
	}

	if config.MaxParallelPipelines == 0 {
		config.MaxParallelPipelines = int64(runtime.NumCPU())

		log.Warningf(
			nil,
			"max_parallel_pipelines is not specified, number of cpu will be used instead: %d",
			config.MaxParallelPipelines,
		)
	}

	return &config, nil
}

func isFileExists(path string) bool {
	stat, err := os.Stat(path)
	return !os.IsNotExist(err) && !stat.IsDir()
}
