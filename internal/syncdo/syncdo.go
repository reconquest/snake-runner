package syncdo

import "sync"

type Action struct {
	done  bool
	err   error
	mutex sync.Mutex
}

func (action *Action) Do(fn func() error) error {
	action.mutex.Lock()
	defer action.mutex.Unlock()

	if action.done {
		return action.err
	}

	action.done = true

	err := fn()
	if err != nil {
		action.err = err
	}

	return action.err
}
