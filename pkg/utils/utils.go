package utils

import (
	"crypto/sha1"
	"fmt"
	"strconv"
	"time"
)

func Now() time.Time {
	return time.Now().UTC()
}

func UniqHash() string {
	hash := sha1.New()
	hash.Write([]byte(strconv.Itoa(int(time.Now().UnixNano()))))
	block := hash.Sum(nil)

	value := fmt.Sprintf("%x", block)
	if len(value) > 6 {
		return value[0:6]
	}

	return value
}
