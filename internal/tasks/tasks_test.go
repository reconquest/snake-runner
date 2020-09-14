package tasks

import (
	"encoding/json"
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

func TestCloneURL_UnmarshalJSON_AcceptsSupportedMethod(t *testing.T) {
	test := assert.New(t)

	for _, method := range []CloneMethod{
		CloneMethodDefault,
		CloneMethodSSH,
		CloneMethodHTTP,
	} {
		input := CloneURL{
			Method: method,
			SSH:    "ssh://url",
			HTTP:   "http://url",
		}

		body, err := json.Marshal(input)
		test.NoError(err)

		var output CloneURL

		err = json.Unmarshal(body, &output)
		test.NoError(err)

		test.EqualValues(input, output)
	}
}

func TestCloneURL_UnmarshalJSON_RejectsUnsupportedMethod(t *testing.T) {
	test := assert.New(t)

	input := CloneURL{
		Method: "git",
		SSH:    "ssh://url",
		HTTP:   "http://url",
	}

	body, err := json.Marshal(input)
	test.NoError(err)

	var output CloneURL

	err = json.Unmarshal(body, &output)
	test.Error(err)
	test.Contains(err.Error(), "unsupported clone method")
}
