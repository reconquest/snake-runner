package safemap

import (
	"sync"

	"github.com/cheekybits/genny/generic"
)

//go:generate genny -in=safemap.go -out=safemap_gen.go gen "KeyType=string,int ValueType=string,context.CancelFunc,Any"

type (
	KeyType   generic.Type
	ValueType generic.Type
)

type KeyTypeToValueType struct {
	sync.RWMutex
	data map[KeyType]ValueType
}

func NewKeyTypeToValueType() KeyTypeToValueType {
	return KeyTypeToValueType{
		data: map[KeyType]ValueType{},
	}
}

func (safe *KeyTypeToValueType) Load(key KeyType) (ValueType, bool) {
	safe.RLock()
	defer safe.RUnlock()

	value, ok := safe.data[key]
	return value, ok
}

func (safe *KeyTypeToValueType) Store(key KeyType, value ValueType) {
	safe.Lock()
	defer safe.Unlock()

	safe.data[key] = value
}

func (safe *KeyTypeToValueType) LoadOrStore(key KeyType, value ValueType) ValueType {
	safe.Lock()
	defer safe.Unlock()

	existing, ok := safe.data[key]
	if ok {
		return existing
	}

	safe.data[key] = value

	return value
}

func (safe *KeyTypeToValueType) Delete(key KeyType) {
	safe.Lock()
	defer safe.Unlock()

	_, ok := safe.data[key]
	if ok {
		delete(safe.data, key)
	}
}

func (safe *KeyTypeToValueType) Range(fn func(key KeyType, value ValueType) bool) {
	safe.Lock()
	defer safe.Unlock()

	for key, value := range safe.data {
		if !fn(key, value) {
			break
		}
	}
}
