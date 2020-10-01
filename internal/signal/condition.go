package signal

import (
	"sync"
)

type Condition interface {
	Satisfy()
	Unsatisfy()
	Wait() bool
}

func NewCondition() Condition {
	return &condition{
		sync: sync.NewCond(&sync.Mutex{}),
	}
}

type condition struct {
	sync  *sync.Cond
	mutex *sync.RWMutex
	ok    uint8
}

func (condition *condition) Satisfy() {
	condition.sync.L.Lock()
	condition.ok = 1
	condition.sync.Broadcast()
	condition.sync.L.Unlock()
}

func (condition *condition) Unsatisfy() {
	condition.sync.L.Lock()
	condition.ok = 2
	condition.sync.Broadcast()
	condition.sync.L.Unlock()
}

func (condition *condition) Wait() bool {
	condition.sync.L.Lock()
	for condition.ok == 0 {
		condition.sync.Wait()
	}
	condition.sync.L.Unlock()
	return condition.ok == 1
}
