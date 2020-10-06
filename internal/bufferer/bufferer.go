package bufferer

import (
	"bytes"
	"sync"
	"time"

	"github.com/reconquest/snake-runner/internal/audit"
	"github.com/reconquest/snake-runner/internal/utils"
)

var (
	DefaultLogsBufferSize    = 1024
	DefaultLogsBufferTimeout = time.Second * 2
)

//go:generate gonstructor -type Bufferer -init init
type Bufferer struct {
	size     int
	duration time.Duration
	flush    func([]byte)
	pipe     chan []byte   `gonstructor:"-"`
	done     chan struct{} `gonstructor:"-"`

	workers      sync.WaitGroup `gonstructor:"-"`
	workersMutex sync.Mutex     `gonstructor:"-"`

	closed     bool       `gonstructor:"-"`
	closeMutex sync.Mutex `gonstructor:"-"`
}

func (writer *Bufferer) init() {
	writer.pipe = make(chan []byte, 128 /* ?? */)
	writer.done = make(chan struct{})
}

func (writer *Bufferer) Run() {
	defer audit.Go("bufferer")()

	writer.workersMutex.Lock()
	writer.workers.Add(1)
	writer.workersMutex.Unlock()
	defer writer.workers.Done()

	ticker := utils.NewTicker(writer.duration)
	buffer := bytes.NewBuffer(nil)
	for {
		select {
		case text, ok := <-writer.pipe:
			if !ok {
				writer.flush(buffer.Bytes())
				return
			}

			buffer.Write(text)

			if buffer.Len() >= writer.size {
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

		case <-writer.done:
			writer.flush(buffer.Bytes())
			return
		}
	}
}

func (writer *Bufferer) Write(data []byte) (int, error) {
	writer.workersMutex.Lock()
	writer.workers.Add(1)
	writer.workersMutex.Unlock()
	defer writer.workers.Done()

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

	go func() {
		for range writer.pipe {
			// drain it
		}
	}()

	writer.workersMutex.Lock()
	writer.workers.Wait()
	writer.workersMutex.Unlock()

	return nil
}
