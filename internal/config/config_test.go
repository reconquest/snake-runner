package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

var reAddress = regexp.MustCompile(`0x[a-h0-9]+`)

func TestUnmarshal(t *testing.T) {
	test := assert.New(t)

	dir := "../../testdata/config/"
	matches, err := filepath.Glob(dir + "*.yaml")
	if err != nil {
		panic(err)
	}

	for _, match := range matches {
		name := strings.TrimSuffix(filepath.Base(match), ".yaml")

		contents, err := ioutil.ReadFile(match)
		if err != nil {
			panic(err)
		}

		pipeline, pipelineErr := Unmarshal(contents)

		tested := false
		if _, err := os.Stat(dir + name + ".error"); err == nil {
			contents, err := ioutil.ReadFile(dir + name + ".error")
			if err != nil {
				panic(err)
			}

			expectedErr := string(contents)

			test.Equal(expectedErr, pipelineErr)
			tested = true
		} else {
			test.NoError(pipelineErr)
		}

		if _, err := os.Stat(dir + name + ".dump"); err == nil {
			contents, err := ioutil.ReadFile(dir + name + ".dump")
			if err != nil {
				panic(err)
			}

			// encoded, err := json.MarshalIndent(pipeline, "", "    ")
			// if err != nil {
			//    panic(err)
			//}

			encoded := reAddress.ReplaceAllString(spew.Sdump(pipeline), "0x")

			test.EqualValues(string(contents), string(encoded), match)
			tested = true
		}

		if !tested {
			test.Fail("No pass conditions", "No error or dump file for testcase: %s", name)
		}
	}

	_ = test
}
