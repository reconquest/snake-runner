// +build !windows

package shell

import (
	"github.com/reconquest/snake-runner/internal/platform"
)

const (
	PLATFORM = platform.POSIX

	DEFAULT_SHELL   = "sh"
	PREFERRED_SHELL = "bash"
)
