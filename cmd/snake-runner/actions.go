package main

import (
	"os"

	cli "gopkg.in/alecthomas/kingpin.v2"
)

type Actions struct {
	registry map[string]func() error
	mainFn   func() error
}

func (actions *Actions) register(cmd *cli.CmdClause, fn func() error) *Actions {
	if actions.registry == nil {
		actions.registry = map[string]func() error{}
	}

	actions.registry[cmd.FullCommand()] = fn

	return actions
}

func (actions *Actions) main(fn func() error) *Actions {
	actions.mainFn = fn
	return nil
}

func (actions *Actions) dispatch(app *cli.Application) error {
	cmd, err := app.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if actions.registry != nil {
		if fn, ok := actions.registry[cmd]; ok {
			return fn()
		}
	}

	return nil
}
