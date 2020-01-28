package main

import (
	"os"
	"syscall"

	"github.com/docopt/docopt-go"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/sign-go"
)

var (
	version = "[not specified during build]"

	usage = "snake-runner " + version + `

Usage:
  snake-runner [options]
  snake-runner -h | --help
  snake-runner --version

Options:
  -h --help           Show this screen.
  --version           Show version.
  -c --config <path>  Use specified config.
                       [default: /etc/snake-runner/snake-runner.conf]
`
)

type commandLineOptions struct {
	ConfigPathValue string `docopt:"--config"`
}

func main() {
	args, err := docopt.ParseArgs(usage, nil, version)
	if err != nil {
		log.Fatal(err)
	}

	var options commandLineOptions
	err = args.Bind(&options)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof(
		karma.Describe("version", version),
		"starting snake-runner",
	)

	config, err := LoadRunnerConfig(options.ConfigPathValue)
	if err != nil {
		log.Fatal(err)
	}

	if config.Log.Debug {
		log.SetLevel(log.LevelDebug)
	}

	if config.Log.Trace {
		log.SetLevel(log.LevelTrace)
	}

	log.Infof(nil, "runner name: %s", config.Name)

	runner := NewRunner(config)
	runner.Start()

	sign.Notify(func(signal os.Signal) bool {
		log.Warningf(nil, "got signal: %s", signal)
		return false
	}, syscall.SIGINT, syscall.SIGTERM)
}
