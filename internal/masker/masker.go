package masker

import (
	"io"
	"strings"

	"github.com/reconquest/snake-runner/internal/env"
)

type Masker interface {
	Mask(string) string
}

var _ Masker = (*Writer)(nil)

//go:generate gonstructor --type Writer --init init
type Writer struct {
	env      *env.Env
	secrets  []string
	replacer *strings.Replacer `gonstructor:"-"`
	dst      io.WriteCloser
}

func (masker *Writer) init() {
	oldnew := []string{}
	for _, secret := range masker.secrets {
		if value, ok := masker.env.Get(secret); ok {
			lines := strings.Split(value, "\n")

			for _, line := range lines {
				value := strings.TrimSpace(line)
				if value != "" {
					oldnew = append(
						oldnew,
						value,
						strings.Repeat("*", len(value)),
					)
				}
			}

		}
	}

	if len(oldnew) > 0 {
		masker.replacer = strings.NewReplacer(oldnew...)
	}
}

func (masker *Writer) Mask(buf string) string {
	if masker.replacer == nil {
		return buf
	}

	return masker.replacer.Replace(buf)
}

func (masker *Writer) Write(buf []byte) (int, error) {
	if masker.replacer != nil {
		return masker.replacer.WriteString(masker.dst, string(buf))
	}

	return masker.dst.Write(buf)
}

func (masker *Writer) Close() error {
	return masker.dst.Close()
}
