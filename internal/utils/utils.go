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
