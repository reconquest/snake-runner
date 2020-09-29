package mapslice

import (
	"fmt"

	"github.com/reconquest/karma-go"

	"gopkg.in/yaml.v3"
)

type MapSlice struct {
	pairs []*Pair
}

var _ yaml.Unmarshaler = (*MapSlice)(nil)

func FromMap(table map[string]string) *MapSlice {
	result := &MapSlice{}
	for key, value := range table {
		result.pairs = append(result.pairs, &Pair{Key: key, Value: value})
	}
	return result
}

func FromPairs(kv ...string) *MapSlice {
	result := &MapSlice{}
	for i := 0; i < len(kv); i += 2 {
		result.pairs = append(result.pairs, &Pair{Key: kv[i], Value: kv[i+1]})
	}
	return result
}

// Pairs returns slice of pairs. It doesn't return a copy. Be careful with
// re-use of the slice.
func (slice *MapSlice) Pairs() []*Pair {
	if slice == nil {
		return []*Pair{}
	}

	return slice.pairs
}

func (slice *MapSlice) Map() map[string]string {
	if slice == nil {
		return map[string]string{}
	}
	result := map[string]string{}
	for _, pair := range slice.pairs {
		result[pair.Key] = pair.Value
	}
	return result
}

func (slice *MapSlice) UnmarshalYAML(value *yaml.Node) error {
	result, err := New(*value)
	*slice = *result
	return err
}

func (slice *MapSlice) Find(key string) *Pair {
	for i := len(slice.pairs) - 1; i > 0; i-- {
		if slice.pairs[i].Key == key {
			return slice.pairs[i]
		}
	}

	return nil
}

type Pair struct {
	Key   string
	Value string
}

func New(root yaml.Node) (*MapSlice, error) {
	if len(root.Content) == 0 {
		return &MapSlice{pairs: []*Pair{}}, nil
	}

	if root.Kind != yaml.MappingNode && root.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("a map expected but got %s node", stringKind(root.Kind))
	}

	if root.Kind == yaml.DocumentNode {
		next := root.Content[0]
		return New(*next)
	}

	pairs := make([]*Pair, len(root.Content)/2)
	_ = pairs
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		for _, node := range []*yaml.Node{key, value} {
			if node.Kind != yaml.ScalarNode {
				return nil, karma.
					Describe("line", node.Line).
					Describe("column", node.Column).
					Describe("value", node.Value).
					Format(
						nil,
						"expected scalar node but found %s node",
						stringKind(node.Kind),
					)
			}
		}

		// do we actually need to check if the key is not unique?
		pairs[i/2] = &Pair{Key: key.Value, Value: value.Value}
	}

	return &MapSlice{pairs: pairs}, nil
}

func stringKind(kind yaml.Kind) string {
	switch kind {
	case yaml.AliasNode:
		return "alias"
	case yaml.DocumentNode:
		return "document"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.SequenceNode:
		return "sequence"
	}

	return "unknown"
}
