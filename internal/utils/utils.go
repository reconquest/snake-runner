package utils

import (
	"context"
	"math/rand"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/reconquest/karma-go"
)

func Now() time.Time {
	return time.Now().UTC()
}

const symbols = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func RandString(n int) string {
	buffer := make([]byte, n)
	for i := range buffer {
		buffer[i] = symbols[rand.Intn(len(symbols))]
	}
	return string(buffer)
}

type Ticker struct {
	ch       <-chan time.Time
	duration time.Duration
}

func NewTicker(duration time.Duration) *Ticker {
	ticker := &Ticker{duration: duration}
	ticker.Reset()
	return ticker
}

func (ticker *Ticker) Get() <-chan time.Time {
	return ticker.ch
}

func (ticker *Ticker) Reset() {
	ticker.ch = time.After(ticker.duration)
}

func IsDone(context context.Context) bool {
	select {
	case <-context.Done():
		return true
	default:
		return false
	}
}

type causer interface {
	Cause() error
}

type unwraper interface {
	Unwrap() error
}

type Terminator interface {
	Terminate()
}

func IsCanceled(err error) bool {
	if err == context.Canceled {
		return true
	}

	if karma.Contains(err, context.Canceled) {
		return true
	}

	if err, ok := err.(karma.Karma); ok {
		for _, reason := range err.GetReasons() {
			if reason, ok := reason.(error); ok {
				if IsCanceled(reason) {
					return true
				}
			}
		}
	}

	if errdefs.IsCancelled(err) {
		return true
	}

	if err, ok := err.(unwraper); ok {
		return IsCanceled(err.Unwrap())
	}

	if err, ok := err.(causer); ok {
		return IsCanceled(err.Cause())
	}

	return false
}
