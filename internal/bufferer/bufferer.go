package bufferer

import (
	"bytes"
	"sync"
	"time"

	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/utils"
)

var (
	DefaultFlushSize     = 1024
	DefaultFlushInterval = time.Second * 2
	DefaultChanSize      = 128
)

//go:generate gonstructor -type Bufferer -init init
type Bufferer struct {
	chanSize      int
	flushSize     int
	flushInterval time.Duration
	flush         func([]byte)
	pipe          chan []byte   `gonstructor:"-"`
	done          chan struct{} `gonstructor:"-"`

	run      sync.WaitGroup `gonstructor:"-"`
	runMutex sync.Mutex     `gonstructor:"-"`

	writers      sync.WaitGroup `gonstructor:"-"`
	writersMutex sync.Mutex     `gonstructor:"-"`

	closed     bool       `gonstructor:"-"`
	closeMutex sync.Mutex `gonstructor:"-"`
}

func (writer *Bufferer) init() {
	writer.pipe = make(chan []byte, writer.chanSize)
	writer.done = make(chan struct{})
}

func (writer *Bufferer) Run() {
	defer audit.Go("bufferer")()

	writer.runMutex.Lock()
	writer.run.Add(1)
	writer.runMutex.Unlock()
	defer writer.run.Done()

	ticker := utils.NewTicker(writer.flushInterval)
	buffer := bytes.NewBuffer(nil)
	for {
		select {
		case text, ok := <-writer.pipe:
			if !ok {
				if buffer.Len() != 0 {
					writer.flush(buffer.Bytes())
				}
				return
			}

			buffer.Write(text)

			if buffer.Len() >= writer.flushSize {
				writer.flush(buffer.Bytes())
				buffer.Reset()

				ticker.Reset()
			}

		case <-ticker.Get():
			if buffer.Len() != 0 {
				writer.flush(buffer.Bytes())
				buffer.Reset()
			}

			ticker.Reset()
		}
	}
}

func (writer *Bufferer) Write(data []byte) (int, error) {
	writer.writersMutex.Lock()
	writer.writers.Add(1)
	writer.writersMutex.Unlock()
	defer writer.writers.Done()

	select {
	case <-writer.done:
		return len(data), nil
	default:
	}

	select {
	case <-writer.done:
	case writer.pipe <- data:
	}

	return len(data), nil
}

func (writer *Bufferer) Close() error {
	writer.closeMutex.Lock()
	defer writer.closeMutex.Unlock()

	if writer.closed {
		return nil
	}

	writer.closed = true

	close(writer.done)

	writer.writersMutex.Lock()
	writer.writers.Wait()
	writer.writersMutex.Unlock()

	close(writer.pipe)

	writer.runMutex.Lock()
	writer.run.Wait()
	writer.runMutex.Unlock()

	return nil
}
