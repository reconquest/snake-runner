// +build windows

package runner

import (
	"os"
	"path/filepath"
)

var (
	DEFAULT_ACCESS_TOKEN_PATH = filepath.Join(os.Getenv("ProgramData"), "snake-runner", "secrets", "access_token")
	DEFAULT_PIPELINES_DIR     = filepath.Join(os.Getenv("ProgramData"), "snake-runner", "pipelines")
	DEFAULT_CONFIG_PATH       = filepath.Join(os.Getenv("ProgramData"), "snake-runner", "config", "snake-runner.conf")
)
