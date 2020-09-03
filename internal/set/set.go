package set

import (
	"sync"

	"github.com/cheekybits/genny/generic"
)

//go:generate genny -in=set.go -out=set_gen.go gen "Type=string,int,*exec.Cmd"

type (
	Type generic.Type
)

type TypeSet struct {
	mutex sync.RWMutex
	items map[Type]struct{}
}

func NewTypeSet(values ...Type) *TypeSet {
	set := &TypeSet{
		items: map[Type]struct{}{},
	}
	for _, value := range values {
		set.Put(value)
	}
	return set
}

func (set *TypeSet) Has(value Type) bool {
	set.mutex.RLock()
	_, ok := set.items[value]
	set.mutex.RUnlock()
	return ok
}

func (set *TypeSet) Put(value Type) {
	set.mutex.Lock()
	set.items[value] = struct{}{}
	set.mutex.Unlock()
}

func (set *TypeSet) Delete(value Type) {
	set.mutex.Lock()
	delete(set.items, value)
	set.mutex.Unlock()
}

func (set *TypeSet) List() []Type {
	slice := make([]Type, len(set.items))
	i := 0
	set.mutex.RLock()
	for value := range set.items {
		slice[i] = value
		i++
	}
	set.mutex.RUnlock()
	return slice
}
