package main

import (
	"net/http"
	"os"
	"syscall"

	"github.com/docopt/docopt-go"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/sign-go"
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

	config, err := LoadConfig(options.ConfigPathValue)
	if err != nil {
		log.Fatal(err)
	}

	runner := NewRunner(config)
	go runner.Start()

	handler := NewWebHandler(runner)

	server := http.Server{
		Addr:    config.ListenAddress,
		Handler: handler,
	}

	go func() {
		log.Infof(
			karma.Describe("address", config.ListenAddress),
			"starting http server",
		)

		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf(
				err,
				"unable to listen and serve on %s",
				config.ListenAddress,
			)
		}
	}()

	sign.Notify(func(signal os.Signal) bool {
		log.Infof(
			karma.Describe("signal", signal.String()),
			"observed a system signal, shutting down",
		)

		err := server.Close()
		if err != nil {
			log.Errorf(err, "unable to gracefuly shutdown http server")
		}

		return false
	}, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
}
