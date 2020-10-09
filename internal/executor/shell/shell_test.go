package shell

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/assert"
	"github.com/reconquest/snake-runner/internal/executor"
)

func TestShell_Exec_Bug(t *testing.T) {
	test := assert.New(t)

	shell := NewShell()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	box, err := shell.Create(ctx, executor.CreateOptions{Name: "test"})
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result := ""

			id := "x" + strings.Repeat(fmt.Sprint(i), 1) + "x"

			err = shell.Exec(ctx, box, executor.ExecOptions{
				Cmd:          []string{"bash", "-c", "echo " + id},
				AttachStderr: true,
				AttachStdout: true,
				OutputConsumer: func(output string) {
					result += output
				},
			})
			if err != nil {
				panic(err)
			}

			test.Contains(result, id)
		}(i)
	}

	wg.Wait()

	err = shell.Destroy(ctx, box)
	if err != nil {
		panic(err)
	}
}
