package masker

import (
	"io"
	"sort"
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
	old := []string{}
	for _, secret := range masker.secrets {
		if value, ok := masker.env.Get(secret); ok {
			lines := strings.Split(value, "\n")
			for _, line := range lines {
				value := strings.TrimSpace(line)
				if value != "" {
					old = append(old, value)
				}
			}

		}
	}

	sort.Slice(old, func(i, j int) bool {
		return len(old[i]) > len(old[j])
	})

	old = unique(old)

	oldnew := make([]string, len(old)*2)
	for i, item := range old {
		oldnew[i*2] = item
		oldnew[i*2+1] = strings.Repeat("*", len(item))
	}

	if len(oldnew) > 0 {
		masker.replacer = strings.NewReplacer(oldnew...)
	}
}

func unique(slice []string) []string {
	for i := 0; i < len(slice)-1; i++ {
		if slice[i] == slice[i+1] {
			slice = append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
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
