// Code generated by gonstructor --type Writer --init init; DO NOT EDIT.

package masker

import (
	"io"

	"github.com/reconquest/snake-runner/internal/env"
)

func NewWriter(
	env *env.Env,
	secrets []string,
	dst io.WriteCloser,
) *Writer {
	r := &Writer{
		env:     env,
		secrets: secrets,
		dst:     dst,
	}

	r.init()

	return r
}
