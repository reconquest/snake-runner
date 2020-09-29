package mapslice

import (
	"testing"

	"github.com/alecthomas/assert"
	"github.com/reconquest/karma-go"
	"gopkg.in/yaml.v3"
)

func node(data string) yaml.Node {
	var result yaml.Node
	err := yaml.Unmarshal([]byte(data), &result)
	if err != nil {
		panic(karma.Format(err, data))
	}
	return result
}

func TestMapslice(t *testing.T) {
	test := assert.New(t)
	_ = test

	contents := `
q: 1q
w: 1w
e: 1e
r: 1r
t: 1t
y: 1y
u: 1u
i: 1i
o: 1o
p: 1p
a: 1a
s: 1s
d: 1d
f: 1f
g: 1g
h: 1h
j: 1j
k: 1k
l: 1l
z: 1z
x: 1x
c: 1c
v: 1v
b: 1b
n: 1n
m: 1m
`
	expect := []string{
		"q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "a", "s", "d",
		"f", "g", "h", "j", "k", "l", "z", "x", "c", "v", "b", "n", "m",
	}

	ms, err := New(node(contents))
	if err != nil {
		panic(err)
	}

	keys := []string{}
	for _, pair := range ms.Pairs() {
		keys = append(keys, pair.Key)
		test.EqualValues("1"+pair.Key, pair.Value)
	}

	test.EqualValues(expect, keys)

	test.EqualValues("1d", ms.Find("d").Value)
	test.EqualValues("d", ms.Find("d").Key)
	test.Nil(nil, ms.Find("no"))
}

func TestMapslice_InvalidKey(t *testing.T) {
	test := assert.New(t)
	_ = test

	contents := `
q: 1q
[a]: b
`
	ms, err := New(node(contents))
	test.Nil(ms)
	test.Error(err)
	test.Contains(err.Error(), "scalar node")
	test.Contains(err.Error(), "sequence node")
}

func TestMapslice_InvalidValue(t *testing.T) {
	test := assert.New(t)
	_ = test

	contents := `
q: 1q
w: 
  - x
`
	ms, err := New(node(contents))
	test.Nil(ms)
	test.Error(err)
	test.Contains(err.Error(), "scalar node")
	test.Contains(err.Error(), "sequence node")
}

func TestMapslice_UnmarshalYAML(t *testing.T) {
	test := assert.New(t)

	contents := `
v:
  x: 1
`
	var config struct {
		V *MapSlice
	}

	err := yaml.Unmarshal([]byte(contents), &config)
	if err != nil {
		panic(err)
	}

	test.NotNil(config.V)
	test.Len(config.V.Pairs(), 1)
}
