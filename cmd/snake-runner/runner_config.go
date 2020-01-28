package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/kovetskiy/ko"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

type RunnerConfig struct {
	// it's actually required but it will be handled manually
	MasterAddress string `yaml:"master_address" env:"SNAKE_MASTER_ADDRESS"`

	Log struct {
		Debug bool `yaml:"debug" env:"SNAKE_LOG_DEBUG"`
		Trace bool `yaml:"trace" env:"SNAKE_LOG_TRACE"`
	}

	Name string `yaml:"name" env:"SNAKE_NAME"`

	RegistrationToken string `yaml:"registration_token" env:"SNAKE_REGISTRATION_TOKEN"`

	AccessToken     string `yaml:"access_token" env:"SNAKE_ACCESS_TOKEN"`
	AccessTokenPath string `yaml:"access_token_path" env:"SNAKE_ACCESS_TOKEN_PATH" default:"/var/lib/snake-runner/secrets/access_token"`

	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" default:"30s"`
	SchedulerInterval time.Duration `yaml:"scheduler_interval" default:"5s"`

	Virtualization       string `yaml:"virtualization" default:"docker" env:"SNAKE_VIRTUALIZATION" required:"true"`
	MaxParallelPipelines int64  `yaml:"max_parallel_pipelines" env:"SNAKE_MAX_PARALLEL_PIPELINES" default:"0" required:"true"`

	PipelinesDir string `yaml:"pipelines_dir" env:"SNAKE_PIPELINES_DIR" default:"/var/lib/snake-runner/pipelines" required:"true"`

	Docker struct {
		Network string `yaml:"network" env:"SNAKE_DOCKER_NETWORK"`
	} `yaml:"docker"`
}

func LoadRunnerConfig(path string) (*RunnerConfig, error) {
	log.Infof(karma.Describe("path", path), "loading configuration")

	var config RunnerConfig
	err := ko.Load(path, &config, yaml.Unmarshal)
	if err != nil {
		return nil, err
	}

	if config.MasterAddress == "" || config.RegistrationToken == "" {
		ShowMessageNotConfigured(config)
		os.Exit(1)
	}

	if config.AccessTokenPath != "" && config.AccessToken == "" {
		tokenData, err := ioutil.ReadFile(config.AccessTokenPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, karma.Format(
				err,
				"unable to read specified token file: %s", config.AccessTokenPath,
			)
		}

		config.AccessToken = strings.TrimSpace(string(tokenData))
	}

	if config.Virtualization == "none" {
		log.Warningf(nil, "No virtualization is used, all commands will be "+
			"executed on the local host with current permissions")
	}

	if config.MaxParallelPipelines == 0 {
		config.MaxParallelPipelines = int64(runtime.NumCPU())

		log.Warningf(
			nil,
			"max_parallel_pipelines is not specified, number of CPU will be used instead: %d",
			config.MaxParallelPipelines,
		)
	}

	if config.Name == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, karma.Format(err, "unable to obtain hostname")
		}

		config.Name = hostname
	}

	if !filepath.IsAbs(config.PipelinesDir) {
		config.PipelinesDir, err = filepath.Abs(config.PipelinesDir)
		if err != nil {
			return nil, karma.Format(
				err,
				"unable to get absolute path of %s", config.PipelinesDir,
			)
		}
	}

	return &config, nil
}
