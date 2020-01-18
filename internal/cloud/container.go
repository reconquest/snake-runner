package cloud

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
)

type Container struct {
	Name string
	ID   string
}

func (state *ContainerState) GetError() error {
	data := []string{}
	if state.ExitCode != 0 {
		data = append(data, fmt.Sprintf("exit code: %d", state.ExitCode))
	}
	if state.Error != "" {
		data = append(data, fmt.Sprintf("error: %s", state.Error))
	}
	if state.OOMKilled {
		data = append(data, "killed by oom")
	}
	if len(data) > 0 {
		return fmt.Errorf("%s", strings.Join(data, "; "))
	}

	return nil
}

type ContainerState struct {
	types.ContainerState
}
