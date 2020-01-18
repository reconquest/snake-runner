package cloud

import (
	"sync"
)

type valueMutex struct {
	value bool
	sync.Mutex
}

type mapMutex struct {
	sync.Map
}

func (themap *mapMutex) get(key string) *valueMutex {
	value, _ := themap.LoadOrStore(key, &valueMutex{})
	return value.(*valueMutex)
}
