package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/docopt/docopt-go"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/sign-go"
	"github.com/reconquest/snake-runner/internal/audit"
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

	// uncomment if you want to trace goroutines
	//
	// go func() {
	//    defer audit.Go("audit", "watcher")()

	//    for {
	//        num := audit.NumGoroutines()
	//        log.Tracef(nil, "{audit} goroutines audit: %d runtime: %d", num, runtime.NumGoroutine())

	//        time.Sleep(time.Millisecond * 3000)
	//    }
	//}()

	go sign.Notify(func(signal os.Signal) bool {
		defer audit.Go("audit", "sighup")()

		routines := audit.Goroutines()

		log.Warningf(nil, "{audit} goroutines: %d", len(routines))
		for _, routine := range routines {
			log.Warningf(nil, "{audit} "+routine)
		}

		return true
	}, syscall.SIGHUP)

	interrupts := make(chan os.Signal, 0)
	signal.Notify(interrupts, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case <-runner.Terminated():
		// exit

	case signal := <-interrupts:
		log.Warningf(nil, "got signal: %s, shutting down runner", signal)
	}

	runner.Shutdown()
	log.Warningf(nil, "shutdown: runner gracefully terminated")
}
