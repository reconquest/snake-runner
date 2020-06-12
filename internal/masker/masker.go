package masker

import (
	"io"
	"strings"

	"github.com/reconquest/snake-runner/internal/env"
)

//go:generate gonstructor --type Masker --init init
type Masker struct {
	env      *env.Env
	secrets  []string
	replacer *strings.Replacer `gonstructor:"-"`
	dst      io.WriteCloser
}

func (masker *Masker) init() {
	oldnew := []string{}
	for _, secret := range masker.secrets {
		if value, ok := masker.env.Get(secret); ok {
			lines := strings.Split(value, "\n")

			// we can't mask multi-line strings since we use lineflushwriter
			// that can only guarantee one line
			if len(lines) > 1 {
				continue
			}

			value := lines[0]
			if strings.TrimSpace(value) != "" {
				oldnew = append(oldnew, value, strings.Repeat("*", len(value)))
			}
		}
	}

	if len(oldnew) > 0 {
		masker.replacer = strings.NewReplacer(oldnew...)
	}
}

func (masker *Masker) Write(buf []byte) (int, error) {
	if masker.replacer != nil {
		return masker.replacer.WriteString(masker.dst, string(buf))
	}

	return masker.dst.Write(buf)
}

func (masker *Masker) Close() error {
	return masker.dst.Close()
}
