// This file was automatically generated by genny.
// Any changes will be lost if this file is regenerated.
// see https://github.com/cheekybits/genny

package set

import (
	"os/exec"
	"sync"
)

type ()

type StringSet struct {
	mutex sync.RWMutex
	items map[string]struct{}
}

func NewStringSet(values ...string) *StringSet {
	set := &StringSet{
		items: map[string]struct{}{},
	}
	for _, value := range values {
		set.Put(value)
	}
	return set
}

func (set *StringSet) Has(value string) bool {
	set.mutex.RLock()
	_, ok := set.items[value]
	set.mutex.RUnlock()
	return ok
}

func (set *StringSet) Put(value string) {
	set.mutex.Lock()
	set.items[value] = struct{}{}
	set.mutex.Unlock()
}

func (set *StringSet) Delete(value string) {
	set.mutex.Lock()
	delete(set.items, value)
	set.mutex.Unlock()
}

func (set *StringSet) List() []string {
	slice := make([]string, len(set.items))
	i := 0
	set.mutex.RLock()
	for value := range set.items {
		slice[i] = value
		i++
	}
	set.mutex.RUnlock()
	return slice
}

type ()

type IntSet struct {
	mutex sync.RWMutex
	items map[int]struct{}
}

func NewIntSet(values ...int) *IntSet {
	set := &IntSet{
		items: map[int]struct{}{},
	}
	for _, value := range values {
		set.Put(value)
	}
	return set
}

func (set *IntSet) Has(value int) bool {
	set.mutex.RLock()
	_, ok := set.items[value]
	set.mutex.RUnlock()
	return ok
}

func (set *IntSet) Put(value int) {
	set.mutex.Lock()
	set.items[value] = struct{}{}
	set.mutex.Unlock()
}

func (set *IntSet) Delete(value int) {
	set.mutex.Lock()
	delete(set.items, value)
	set.mutex.Unlock()
}

func (set *IntSet) List() []int {
	slice := make([]int, len(set.items))
	i := 0
	set.mutex.RLock()
	for value := range set.items {
		slice[i] = value
		i++
	}
	set.mutex.RUnlock()
	return slice
}

type ()

type ExecCmdSet struct {
	mutex sync.RWMutex
	items map[*exec.Cmd]struct{}
}

func NewExecCmdSet(values ...*exec.Cmd) *ExecCmdSet {
	set := &ExecCmdSet{
		items: map[*exec.Cmd]struct{}{},
	}
	for _, value := range values {
		set.Put(value)
	}
	return set
}

func (set *ExecCmdSet) Has(value *exec.Cmd) bool {
	set.mutex.RLock()
	_, ok := set.items[value]
	set.mutex.RUnlock()
	return ok
}

func (set *ExecCmdSet) Put(value *exec.Cmd) {
	set.mutex.Lock()
	set.items[value] = struct{}{}
	set.mutex.Unlock()
}

func (set *ExecCmdSet) Delete(value *exec.Cmd) {
	set.mutex.Lock()
	delete(set.items, value)
	set.mutex.Unlock()
}

func (set *ExecCmdSet) List() []*exec.Cmd {
	slice := make([]*exec.Cmd, len(set.items))
	i := 0
	set.mutex.RLock()
	for value := range set.items {
		slice[i] = value
		i++
	}
	set.mutex.RUnlock()
	return slice
}
