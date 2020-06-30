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
	thread   sync.WaitGroup `gonstructor:"-"`
	size     int
	duration time.Duration
	flush    func([]byte)
	pipe     chan []byte `gonstructor:"-"`
}

func (writer *Bufferer) init() {
	writer.pipe = make(chan []byte, 128 /* ?? */)
}

func (writer *Bufferer) Run() {
	defer audit.Go("Bufferer")()

	writer.thread.Add(1)
	defer writer.thread.Done()

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
		}
	}
}

func (writer *Bufferer) Write(data []byte) (int, error) {
	writer.pipe <- data
	return len(data), nil
}

func (writer *Bufferer) Close() error {
	close(writer.pipe)
	return nil
}

func (writer *Bufferer) Wait() {
	writer.thread.Wait()
}
