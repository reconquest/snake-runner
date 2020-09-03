package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/set"
	"github.com/reconquest/snake-runner/internal/spawner"
	"github.com/reconquest/snake-runner/internal/utils"
)

var _ spawner.Spawner = (*Shell)(nil)

type Shell struct {
}

type Box struct {
	id        string
	processes *set.ExecCmdSet
}

func (box *Box) String() string {
	return box.id
}

func (box *Box) ID() string {
	return box.id
}

func NewShell() (*Shell, error) {
	return &Shell{}, nil
}

func (shell *Shell) Type() spawner.SpawnerType {
	return spawner.SPAWNER_SHELL
}

func (shell *Shell) Create(
	ctx context.Context,
	opts spawner.CreateOptions,
) (spawner.Container, error) {
	return &Box{
		id:        opts.Name,
		processes: set.NewExecCmdSet(),
	}, nil
}

func (shell *Shell) Destroy(
	ctx context.Context,
	container spawner.Container,
) error {
	box := box(container)
	cmds := box.processes.List()
	for _, cmd := range cmds {
		fact := karma.Describe("cmd", cmd.Args)
		log.Tracef(fact, "destroying process")

		err := cmd.Process.Kill()
		if err != nil {
			log.Tracef(fact.Describe("error", err), "sent kill signal")
		}
	}
	return nil
}

func (shell *Shell) Exec(
	ctx context.Context,
	container spawner.Container,
	opts spawner.ExecOptions,
) error {
	box := box(container)

	if len(opts.Cmd) == 0 {
		return errors.New("an empty command specified")
	}

	name := opts.Cmd[0]
	args := opts.Cmd[1:]

	workers := &sync.WaitGroup{}

	cmd := exec.CommandContext(ctx, name, args...)
	if opts.AttachStdout {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return karma.Format(
				err,
				"can't pipe stdout",
			)
		}

		defer stdout.Close()

		workers.Add(1)
		go func() {
			defer workers.Done()
			writer := callbackWriter{ctx: ctx, callback: opts.OutputConsumer}
			_, err := io.Copy(writer, stdout)
			if err != nil {
				return
			}
		}()
	}

	if opts.AttachStderr {
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return karma.Format(
				err,
				"can't pipe stderr",
			)
		}

		defer stderr.Close()

		workers.Add(1)
		go func() {
			defer workers.Done()
			writer := callbackWriter{ctx: ctx, callback: opts.OutputConsumer}
			_, err := io.Copy(writer, stderr)
			if err != nil {
				return
			}
		}()
	}

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	cmd.Env = opts.Env
	cmd.Dir = opts.WorkingDir

	err := cmd.Start()
	if err != nil {
		return err
	}

	defer func() {
		box.processes.Delete(cmd)
	}()

	box.processes.Put(cmd)

	return cmd.Wait()
}

func (shell *Shell) Cleanup() error {
	return nil
}

func (shell *Shell) Prepare(
	ctx context.Context,
	opts spawner.PrepareOptions,
) error {
	return nil
}

type callbackWriter struct {
	ctx      context.Context
	callback spawner.OutputConsumer
}

func (callbackWriter callbackWriter) Write(data []byte) (int, error) {
	if callbackWriter.callback == nil {
		return len(data), nil
	}

	if utils.IsDone(callbackWriter.ctx) {
		return 0, context.Canceled
	}

	callbackWriter.callback(string(data))

	return len(data), nil
}

func box(container spawner.Container) *Box {
	box, ok := container.(*Box)
	if !ok {
		panic("BUG: unexpected type given: " + fmt.Sprintf("%T", container))
	}
	return box
}
