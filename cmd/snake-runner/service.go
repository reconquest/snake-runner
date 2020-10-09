package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/kardianos/service"
	"github.com/kovetskiy/ko"
	"github.com/kovetskiy/lorg"
	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/builtin"
	"github.com/reconquest/snake-runner/internal/platform"
	"github.com/reconquest/snake-runner/internal/runner"
)

type ServiceController struct {
	svc service.Service
}

func (ctl *ServiceController) lazyInit() error {
	if ctl.svc == nil {
		if platform.CURRENT == platform.WINDOWS {
			svc, err := service.New(Nop{}, &service.Config{
				Name:        "snake-runner",
				DisplayName: "snake-runner",
				Description: "Runs Pipelines & Jobs provided by Snake CI",
				Executable:  os.Args[0],
				Arguments:   []string{"service", "run", "--config", *configPath},
			})
			if err != nil {
				return err
			}

			ctl.svc = svc

			return nil
		}

		return errors.New(
			"The service commands are not yet implemented for linux",
		)
	}

	return nil
}

func (ctl *ServiceController) Install() error {
	if err := ctl.lazyInit(); err != nil {
		return err
	}

	cfg, err := runner.LoadConfig(*configPath, ko.RequireFile(true))
	if err != nil {
		if err == runner.ErrorNotConfigured {
			runner.ShowMessageNotInstalledNotConfigured(cfg)
			os.Exit(1)
		}

		return karma.Format(
			err,
			"unable to load & validate config",
		)
	}

	_, err = NewProbeFactory(cfg).Probe()
	if err != nil {
		return err
	}

	log.Info("Installing Snake Runner as a system service")

	err = service.Control(ctl.svc, "install")
	if err != nil {
		return karma.Format(
			err,
			"unable to install Snake Runner as a system service",
		)
	}

	log.Info("Snake Runner has been installed as a system service")

	return nil
}

func (ctl *ServiceController) Uninstall() error {
	if err := ctl.lazyInit(); err != nil {
		return err
	}

	log.Info("Uninstalling Snake Runner as a system service")

	err := service.Control(ctl.svc, "uninstall")
	if err != nil {
		return karma.Format(
			err,
			"unable to uninstall Snake Runner as a system service",
		)
	}

	log.Info("Snake Runner has been uninstall as a system service")

	return nil
}

func (ctl *ServiceController) Start() error {
	if err := ctl.lazyInit(); err != nil {
		return err
	}

	log.Info("Starting the Snake Runner system service")

	err := service.Control(ctl.svc, "start")
	if err != nil {
		return karma.Format(
			err,
			"unable to start Snake Runner as a system service",
		)
	}

	log.Info("Snake Runner has been started")

	return nil
}

func (ctl *ServiceController) Stop() error {
	if err := ctl.lazyInit(); err != nil {
		return err
	}

	log.Info("Stopping the Snake Runner system service")

	err := service.Control(ctl.svc, "stop")
	if err != nil {
		return karma.Format(
			err,
			"unable to stop Snake Runner as a system service",
		)
	}

	log.Info("Snake Runner has been stopped")

	return nil
}

func (ctl *ServiceController) Status() error {
	if err := ctl.lazyInit(); err != nil {
		return err
	}

	status, err := ctl.svc.Status()
	if err != nil {
		return err
	}

	switch status {
	case service.StatusRunning:
		fmt.Println("running")
	case service.StatusStopped:
		fmt.Println("stopped")
	default:
		fmt.Println("unknown")
	}

	return nil
}

func (ctl *ServiceController) Run() (chan struct{}, error) {
	if err := ctl.lazyInit(); err != nil {
		return nil, err
	}

	systemLogger, err := ctl.svc.SystemLogger(nil)
	if err != nil {
		log.Errorf(err, "unable to setup the system logger")
	} else {
		err = systemLogger.Infof("snake-runner %s starting", builtin.Version)

		log.GetLogger().SetSender(func(level lorg.Level, event karma.Hierarchical) error {
			text := level.String() + " " + event.String()
			switch level {
			case lorg.LevelError, lorg.LevelFatal:
				err = systemLogger.Error(text)
			case lorg.LevelWarning:
				err = systemLogger.Warning(text)
			default:
				err = systemLogger.Info(text)
			}

			if err != nil {
				fmt.Fprintln(os.Stderr, "unable to send logs to system log:", err)
			}

			return nil
		})
	}

	stopped := make(chan struct{})
	go func() {
		err = ctl.svc.Run()
		if err != nil {
			log.Errorf(
				err,
				"unable to start the program as a system service",
			)
		} else {
			close(stopped)
		}
	}()

	return stopped, nil
}

type Nop struct{}

func (Nop) Start(_ service.Service) error {
	return nil
}

func (Nop) Stop(_ service.Service) error {
	return nil
}
