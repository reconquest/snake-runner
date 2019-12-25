package main

import (
	"github.com/docopt/docopt-go"
	"github.com/reconquest/pkg/log"
)

var (
	version = "[manual build]"
	usage   = "snake-runner " + version + `

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

	config, err := LoadRunnerConfig(options.ConfigPathValue)
	if err != nil {
		log.Fatal(err)
	}

	if config.Log.Debug {
		log.SetDebug(true)
	}

	runner := NewRunner(config)
	runner.Start()
}
