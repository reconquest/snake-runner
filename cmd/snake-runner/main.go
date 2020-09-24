package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/docopt/docopt-go"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/builtin"
	"github.com/reconquest/snake-runner/internal/runner"
)

var usage = "snake-runner " + builtin.Version + `

Usage:
  snake-runner [options]
  snake-runner [options] service install
  snake-runner [options] service start
  snake-runner [options] service status
  snake-runner [options] service stop
  snake-runner [options] service uninstall
  snake-runner -h | --help
  snake-runner --version

Options:
  -h --help           Show this screen.
  --version           Show version.
  -c --config <path>  Use specified config.
                       [default: ` + runner.DEFAULT_CONFIG_PATH + `]
`

type RunOptions struct {
	Config    string
	Service   bool
	Start     bool
	Install   bool
	Status    bool
	Stop      bool
	Uninstall bool
}

func main() {
	args, err := docopt.ParseArgs(usage, nil, builtin.Version)
	if err != nil {
		log.Fatal(err)
	}

	var options RunOptions
	err = args.Bind(&options)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof(
		karma.Describe("version", builtin.Version),
		"starting snake-runner",
	)

	if options.Service {
		err := controlService(options)
		if err != nil {
			log.Fatal(err)
		}

		return
	}

	config, err := runner.LoadConfig(options.Config)
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

	if os.Getenv("SNAKE_AUDIT_GOROUTINES") == "1" {
		audit.Start()
	}

	snake := NewSnake(config)
	snake.Start()

	interrupts := make(chan os.Signal)
	signal.Notify(interrupts, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-snake.Terminated():
		// exit

	case signal := <-interrupts:
		log.Warningf(nil, "got signal: %s, shutting down runner", signal)
	}

	snake.Shutdown()
	log.Warningf(nil, "shutdown: runner gracefully terminated")
}
