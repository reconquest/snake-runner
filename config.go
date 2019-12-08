package main

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/kovetskiy/ko"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

type Config struct {
	ListenAddress string `yaml:"listen_address"  env:"SNAKE_LISTEN_ADDRESS" required:"true" default:":8585" `
	MasterAddress string `yaml:"master_address" env:"SNAKE_MASTER_ADDRESS" required:"true"`

	Log struct {
		Debug bool `yaml:"trace" env:"SNAKE_LOG_DEBUG"`
	}

	Name string `yaml:"name" env:"SNAKE_NAME"`

	Token     string `yaml:"token" env:"SNAKE_TOKEN"`
	TokenPath string `yaml:"token_path" env:"SNAKE_TOKEN_PATH" default:"/etc/snake-runner/token"`

	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" default:"5s"`
	SchedulerInterval time.Duration `yaml:"scheduler_interval" default:"5s"`
}

func LoadConfig(path string) (*Config, error) {
	log.Infof(karma.Describe("path", path), "loading configuration")

	var config Config
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

	return &config, nil
}
