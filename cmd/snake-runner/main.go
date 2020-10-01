package main

import (
	"os"
	"os/signal"
	"syscall"

	cli "gopkg.in/alecthomas/kingpin.v2"

	"github.com/kovetskiy/ko"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/builtin"
	"github.com/reconquest/snake-runner/internal/runner"
)

var configPath *string

func main() {
	log.Infof(karma.Describe("version", builtin.Version), "starting snake-runner")

	var svcctl ServiceController

	app := cli.New(
		"snake-runner",
		"Snake Runner is a part of Snake CI. "+
			"It handles the workload by running pipelines & jobs.",
	).Version(builtin.Version)

	configPath = app.Flag("config", "Use the given configuration file").
		Short('c').
		Default(runner.DEFAULT_CONFIG_PATH).
		String()

	var actions Actions
	svc := app.Command(
		"service",
		"Control the system service",
	)

	actions.register(
		svc.Command("install", "Install the system service"),
		svcctl.Install,
	)
	actions.register(
		svc.Command("start", "Start the system service"),
		svcctl.Start,
	)
	actions.register(
		svc.Command("stop", "Stop the system service"),
		svcctl.Stop,
	)
	actions.register(
		svc.Command("status", "Get a status of the system service"),
		svcctl.Status,
	)
	actions.register(
		svc.Command("uninstall", "Uninstall the system service"),
		svcctl.Uninstall,
	)
	actions.register(
		svc.Command("run", "Run as the system service").Hidden(),
		func() error {
			shutdown, err := svcctl.Run()
			if err != nil {
				return err
			}

			return run(shutdown)
		},
	)
	actions.register(
		app.Command("run", "Run pipelines & jobs").Hidden().Default(),
		func() error {
			return run(make(chan struct{}))
		},
	)

	err := actions.dispatch(app)
	if err != nil {
		log.Fatal(err)
	}
}

func run(serviceShutdown chan struct{}) error {
	config, err := runner.LoadConfig(*configPath, ko.RequireFile(false))
	if err != nil {
		if err == runner.ErrorNotConfigured {
			runner.ShowMessageInstalledButNotConfigured(config)
			os.Exit(1)
		}

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

	case <-serviceShutdown:
		log.Warningf(nil, "system service stopped, shutting down runner")

	case signal := <-interrupts:
		log.Warningf(nil, "got signal: %s, shutting down runner", signal)
	}

	snake.Shutdown()
	log.Warningf(nil, "shutdown: runner gracefully terminated")

	return nil
}
