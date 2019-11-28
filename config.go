package main

import (
	"time"

	"github.com/kovetskiy/ko"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

type Config struct {
	ListenAddress string `yaml:"listen_address"  env:"SNAKE_RUNNER_LISTEN_ADDRESS" required:"true" default:":8585" `
	MasterAddress string `yaml:"connect_address" env:"SNAKE_RUNNER_MASTER_ADDRESS" required:"true"`

	Log struct {
		Debug bool `yaml:"trace" env:"SNAKE_RUNNER_LOG_DEBUG" required:"false"`
	}

	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" default:"5s"`
}

func LoadConfig(path string) (*Config, error) {
	log.Infof(karma.Describe("path", path), "loading configuration")

	var config Config
	err := ko.Load(path, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
