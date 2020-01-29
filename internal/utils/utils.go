package utils

import (
	"math/rand"
	"time"
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

func (ticker *Ticker) Reset() {
	ticker.ch = time.After(ticker.duration)
}
