package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	gosrc      = filepath.Join(os.Getenv("GOPATH"), "src")
	id         int64
	goroutines = map[string]struct{}{}
	mutex      sync.Mutex
)

func Go(token ...interface{}) func() {
	_, filename, line, _ := runtime.Caller(1)

	newID := atomic.AddInt64(&id, 1)

	name := fmt.Sprintf("%05d %s:%d", newID, stripGoSrc(filename), line)
	if len(token) > 0 {
		name += fmt.Sprintf(" | %v", token)
	}

	mutex.Lock()
	goroutines[name] = struct{}{}
	mutex.Unlock()

	return func() {
		mutex.Lock()
		delete(goroutines, name)
		mutex.Unlock()
	}
}

func stripGoSrc(filename string) string {
	return strings.TrimPrefix(strings.TrimPrefix(filename, gosrc), "/")
}

func NumGoroutines() int {
	mutex.Lock()
	defer mutex.Unlock()

	return len(goroutines)
}

func Goroutines() []string {
	mutex.Lock()

	names := []string{}
	for name, _ := range goroutines {
		names = append(names, name)
	}

	mutex.Unlock()

	sort.Strings(names)

	return names
}
