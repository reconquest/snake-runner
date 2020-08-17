package shell

import (
	"context"

	"github.com/reconquest/snake-runner/internal/spawner"
)

type Shell struct {
	//
}

func (shell *Shell) CreateContainer(
	ctx context.Context,
	image,
	name string,
	volumes []string,
) (spawner.Container, error) {
	return nil, nil
}
