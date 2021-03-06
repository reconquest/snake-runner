package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"github.com/reconquest/snake-runner/internal/executor"
	"github.com/reconquest/snake-runner/internal/set"
	"github.com/reconquest/snake-runner/internal/utils"
)

var _ executor.Executor = (*Shell)(nil)

type Shell struct{}

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

func NewShell() *Shell {
	return &Shell{}
}

func (shell *Shell) Type() executor.ExecutorType {
	return executor.EXECUTOR_SHELL
}

func (shell *Shell) Create(
	ctx context.Context,
	opts executor.CreateOptions,
) (executor.Container, error) {
	return &Box{
		id:        opts.Name,
		processes: set.NewExecCmdSet(),
	}, nil
}

func (shell *Shell) Destroy(
	ctx context.Context,
	container executor.Container,
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
	container executor.Container,
	opts executor.ExecOptions,
) error {
	box := box(container)

	if len(opts.Cmd) == 0 {
		return errors.New("an empty command specified")
	}

	name := opts.Cmd[0]
	args := opts.Cmd[1:]

	log.Tracef(nil, "shell exec: %s %s", name, args)

	cmd := exec.CommandContext(ctx, name, args...)

	writer := &callbackWriter{callback: opts.OutputConsumer}
	if opts.AttachStdout {
		cmd.Stdout = writer
	}
	if opts.AttachStderr {
		cmd.Stderr = writer
	}

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	cmd.Env = append(os.Environ(), opts.Env...)
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
	opts executor.PrepareOptions,
) error {
	return nil
}

func (shell *Shell) DetectShell(
	ctx context.Context,
	container executor.Container,
) (string, error) {
	_, err := shell.LookPath(ctx, PREFERRED_SHELL)
	if err != nil {
		return DEFAULT_SHELL, nil
	}

	return PREFERRED_SHELL, nil
}

func (shell *Shell) LookPath(
	ctx context.Context,
	path string,
) (string, error) {
	return exec.LookPath(path)
}

type callbackWriter struct {
	ctx      context.Context
	mutex    sync.Mutex
	callback executor.OutputConsumer
}

func (callbackWriter *callbackWriter) Write(data []byte) (int, error) {
	if callbackWriter.callback == nil {
		return len(data), nil
	}

	if callbackWriter.ctx != nil {
		if utils.IsDone(callbackWriter.ctx) {
			return 0, context.Canceled
		}
	}

	callbackWriter.mutex.Lock()
	defer callbackWriter.mutex.Unlock()
	callbackWriter.callback(string(data))

	return len(data), nil
}

func box(container executor.Container) *Box {
	box, ok := container.(*Box)
	if !ok {
		panic("BUG: unexpected type given: " + fmt.Sprintf("%T", container))
	}
	return box
}
