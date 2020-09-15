package tasks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloneURL_GetPreferredURL_ReturnsSSHByDefault(t *testing.T) {
	test := assert.New(t)

	url := CloneURL{
		SSH:  "ssh://url",
		HTTP: "http://url",
	}

	test.Equal("ssh://url", url.GetPreferredURL())
}

func TestCloneURL_GetPreferredURL_ReturnsSSH(t *testing.T) {
	test := assert.New(t)

	url := CloneURL{
		Method: CloneMethodSSH,
		SSH:    "ssh://url",
		HTTP:   "http://url",
	}

	test.Equal("ssh://url", url.GetPreferredURL())
}

func TestCloneURL_GetPreferredURL_ReturnsHTTP(t *testing.T) {
	test := assert.New(t)

	url := CloneURL{
		Method: CloneMethodHTTP,
		SSH:    "ssh://url",
		HTTP:   "http://url",
	}

	test.Equal("http://url", url.GetPreferredURL())
}

func TestCloneURL_GetPreferredURL_ReturnsSSHIfHTTPIsEmpty(t *testing.T) {
	test := assert.New(t)

	url := CloneURL{
		Method: CloneMethodHTTP,
		SSH:    "ssh://url",
		HTTP:   "",
	}

	test.Equal("ssh://url", url.GetPreferredURL())
}

func TestCloneURL_GetPreferredURL_ReturnsSSHIfMethodIsUnknown(t *testing.T) {
	test := assert.New(t)

	url := CloneURL{
		Method: "git",
		SSH:    "ssh://url",
		HTTP:   "http://url",
	}

	test.Equal("ssh://url", url.GetPreferredURL())
}
