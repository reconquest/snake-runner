package signal

import (
	"sync"
)

type Condition interface {
	Satisfy()
	Wait()
}

func NewCondition() Condition {
	return &condition{
		sync: sync.NewCond(&sync.Mutex{}),
	}
}

type condition struct {
	sync  *sync.Cond
	mutex *sync.RWMutex
	ok    bool
}

func (condition *condition) Satisfy() {
	condition.sync.L.Lock()
	condition.ok = true
	condition.sync.Broadcast()
	condition.sync.L.Unlock()
}

func (condition *condition) Wait() {
	condition.sync.L.Lock()
	for !condition.ok {
		condition.sync.Wait()
	}
	condition.sync.L.Unlock()
}
