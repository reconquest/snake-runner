package sidecar

import (
	"strings"

	"github.com/reconquest/snake-runner/internal/responses"
)

func joinKnownHosts(hosts []responses.KnownHost) string {
	result := make([]string, len(hosts))
	for i, host := range hosts {
		result[i] = host.HostPort() + " " + host.Key
	}

	return strings.Join(result, "\n")
}
