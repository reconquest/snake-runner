package set

import (
	"github.com/cheekybits/genny/generic"
)

//go:generate genny -in=set.go -out=set_gen.go gen "Type=string,int"

type (
	Type generic.Type
)

type TypeSet struct {
	items map[Type]struct{}
}

func NewTypeSet(values ...Type) *TypeSet {
	set := &TypeSet{items: map[Type]struct{}{}}
	for _, value := range values {
		set.Put(value)
	}
	return set
}

func (set *TypeSet) Has(value Type) bool {
	_, ok := set.items[value]
	return ok
}

func (set *TypeSet) Put(value Type) {
	set.items[value] = struct{}{}
}

func (set *TypeSet) Delete(value Type) {
	delete(set.items, value)
}

func (set *TypeSet) List() []Type {
	slice := make([]Type, len(set.items))
	i := 0
	for value := range set.items {
		slice[i] = value
		i++
	}
	return slice
}
