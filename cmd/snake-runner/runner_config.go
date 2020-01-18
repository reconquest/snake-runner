package main

import (
	"fmt"
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
	"github.com/reconquest/snake-runner/internal/sshkey"
)

const (
	defaultBlockSize = 4096
)

type RunnerConfig struct {
	// it's actually required but it will be handled manually
	MasterAddress string `yaml:"master_address" env:"SNAKE_MASTER_ADDRESS"`

	Log struct {
		Debug bool `yaml:"debug" env:"SNAKE_LOG_DEBUG"`
		Trace bool `yaml:"trace" env:"SNAKE_LOG_TRACE"`
	}

	Name string `yaml:"name" env:"SNAKE_NAME"`

	Token     string `yaml:"token" env:"SNAKE_TOKEN"`
	TokenPath string `yaml:"token_path" env:"SNAKE_TOKEN_PATH" default:"/var/lib/snake-runner/secrets/token"`

	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" default:"30s"`
	SchedulerInterval time.Duration `yaml:"scheduler_interval" default:"5s"`

	Virtualization       string `yaml:"virtualization" default:"docker" env:"SNAKE_VIRTUALIZATION" required:"true"`
	MaxParallelPipelines int64  `yaml:"max_parallel_pipelines" env:"SNAKE_MAX_PARALLEL_PIPELINES" default:"0" required:"true"`

	SSHKey       string `yaml:"ssh_key" env:"SNAKE_SSH_KEY_PATH" default:"/var/lib/snake-runner/secrets/id_rsa" required:"true"`
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

	if config.MasterAddress == "" {
		showMessageNoConfiguration()
		os.Exit(1)
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
		log.Warningf(
			nil,
			"SSH key not found, generating it (block size: %d): %s",
			defaultBlockSize,
			config.SSHKey,
		)

		dir := filepath.Base(config.SSHKey)
		err = os.MkdirAll(dir, 0644)
		if err != nil {
			return nil, karma.Format(
				err,
				"unable to make directory for ssh key: %s", dir,
			)
		}

		err = sshkey.GeneratePair(config.SSHKey, defaultBlockSize)
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

func isFileExists(path string) bool {
	stat, err := os.Stat(path)
	return !os.IsNotExist(err) && !stat.IsDir()
}

func showMessageNoConfiguration() {
	fmt.Fprintln(os.Stderr, `

 The snake-runner is ready to start but you have not provided address 
 where you run Bitbucket server with Snake CI addon installed.
 
 There are two ways to fix it:
  1) specify master_address in your config file, by default it is /etc/snake-runner/snake-runner.conf
  2) specify address using the environment variable SNAKE_MASTER_ADDRESS, like as following:
   * SNAKE_MASTER_ADDRESS=http://mybitbucket.company/ snake-runner
     or if you are running snake-runner in docker:
   * docker run -e SNAKE_MASTER_ADDRESS=http://mybitbucket.company/ <other-docker-flags-here>

 See also: <XXXXXXXXX>`)
}

func isDocker() bool {
	contents, err := ioutil.ReadFile("/proc/1/cgroup")
	if err != nil {
		log.Errorf(err, "unable to read /proc/1/cgroup to determine "+
			"is it docker container or not")
	}

	/**
	* A docker container has /docker/ in its /cgroup file
	*
	* / # cat /proc/1/cgroup | grep docker
	* 11:pids:/docker/14f3db3a669169c0b801a3ac99...
	* 10:freezer:/docker/14f3db3a669169c0b801a3ac9...
	* 9:cpu,cpuacct:/docker/14f3db3a669169c0b801a3ac...
	* 8:hugetlb:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 7:perf_event:/docker/14f3db3a669169c0b801a3...
	* 6:devices:/docker/14f3db3a669169c0b801a3ac99f...
	* 5:memory:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 4:blkio:/docker/14f3db3a669169c0b801a3ac99f89e914...
	* 3:cpuset:/docker/14f3db3a669169c0b801a3ac99f89e914a...
	* 2:net_cls,net_prio:/docker/14f3db3a669169c0b801a3ac...
	* 1:name=systemd:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 0::/system.slice/docker.service
	***/
	if strings.Contains(string(contents), "/docker/") {
		return true
	}

	return false
}
