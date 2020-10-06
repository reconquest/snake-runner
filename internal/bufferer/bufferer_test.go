package bufferer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/assert"
)

func TestBufferer_Run_DoesNotCrashWhenClosedTooEarly(t *testing.T) {
	test := assert.New(t)
	_ = test

	bufferer := NewBufferer(100, 100, time.Millisecond*100, func([]byte) {})

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

func TestBufferer_Run_FlushesEverything(t *testing.T) {
	test := assert.New(t)

	flushed := []byte{}
	flushblock := make(chan struct{})
	bufferer := NewBufferer(200, 100, time.Hour /* never */, func(input []byte) {
		select {
		// wait until unblock
		case <-flushblock:
		}
		flushed = append(flushed, input...)
	})

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		bufferer.Run()
	}()

	expected := []byte{}
	for i := 0; i < 200; i++ {
		payload := []byte(" " + fmt.Sprint(i))
		expected = append(expected, payload...)
		bufferer.Write(payload)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		bufferer.Close()
	}()

	close(flushblock)

	wg.Wait()

	test.Equal(expected, flushed)
}

func TestBufferer_Run_FlushesEverything_WithoutLock(t *testing.T) {
	test := assert.New(t)

	flushed := []byte{}
	bufferer := NewBufferer(1, 100, time.Hour /* never */, func(input []byte) {
		flushed = append(flushed, input...)
	})

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		bufferer.Run()
	}()

	expected := []byte{}
	for i := 0; i < 200; i++ {
		payload := []byte(" " + fmt.Sprint(i))
		expected = append(expected, payload...)
		bufferer.Write(payload)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		bufferer.Close()
	}()

	wg.Wait()

	test.Equal(expected, flushed)
}
