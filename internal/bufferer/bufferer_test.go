package bufferer

import (
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/assert"
)

func TestBufferer_Run(t *testing.T) {
	test := assert.New(t)
	_ = test

	bufferer := NewBufferer(100, time.Millisecond*100, func([]byte) {})

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		bufferer.Run()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			bufferer.Write([]byte("a"))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		bufferer.Close()
	}()

	wg.Wait()
}
