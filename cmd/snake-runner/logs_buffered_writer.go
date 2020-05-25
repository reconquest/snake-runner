package main

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

//go:generate gonstructor -type LogsBufferedWriter -init init
type LogsBufferedWriter struct {
	thread   sync.WaitGroup `gonstructor:"-"`
	size     int
	duration time.Duration
	flush    func(text string)
	pipe     chan string `gonstructor:"-"`
}

func (writer *LogsBufferedWriter) init() {
	writer.pipe = make(chan string, 128 /* ?? */)
}

func (writer *LogsBufferedWriter) Run() {
	defer audit.Go("LogsBufferedWriter")()

	writer.thread.Add(1)
	defer writer.thread.Done()

	ticker := utils.NewTicker(writer.duration)
	buffer := bytes.NewBufferString("")
	for {
		select {
		case item, ok := <-writer.pipe:
			if !ok {
				writer.flush(buffer.String())
				return
			}
			buffer.WriteString(item)
			if buffer.Len() >= writer.size {
				writer.flush(buffer.String())
				buffer.Reset()

				ticker.Reset()
			}

		case <-ticker.Get():
			if buffer.Len() != 0 {
				writer.flush(buffer.String())
				buffer.Reset()
			}
			ticker.Reset()
		}
	}
}

func (writer *LogsBufferedWriter) stop() {
}

func (writer *LogsBufferedWriter) Write(text string) {
	writer.pipe <- text
}

func (writer *LogsBufferedWriter) Close() {
	close(writer.pipe)
}

func (writer *LogsBufferedWriter) Wait() {
	writer.thread.Wait()
}
