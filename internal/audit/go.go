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
	"syscall"
	"time"

	"github.com/reconquest/pkg/log"
	"github.com/reconquest/sign-go"
	"github.com/reconquest/snake-runner/internal/platform"
)

var (
	gosrc      = filepath.Join(os.Getenv("GOPATH"), "src")
	id         int64
	goroutines = map[string]struct{}{}
	mutex      sync.Mutex

	enabled = false
)

func Start() {
	enabled = true

	go func() {
		defer Go("audit", "watcher")()

		for {
			num := NumGoroutines()
			log.Tracef(
				nil,
				"{audit} goroutines audit: %d runtime: %d",
				num,
				runtime.NumGoroutine(),
			)

			if platform.CURRENT == platform.WINDOWS {
				for _, routine := range Goroutines() {
					log.Warningf(nil, "{audit} "+routine)
				}
			}

			time.Sleep(time.Millisecond * 3000)
		}
	}()

	go sign.Notify(func(_ os.Signal) bool {
		defer Go("audit", "sighup")()

		routines := Goroutines()

		log.Warningf(nil, "{audit} goroutines: %d", len(routines))
		for _, routine := range routines {
			log.Warningf(nil, "{audit} "+routine)
		}

		return true
	}, syscall.SIGHUP)
}

func noop() {}

func Go(token ...interface{}) func() {
	if !enabled {
		return noop
	}

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
	for name := range goroutines {
		names = append(names, name)
	}

	mutex.Unlock()

	sort.Strings(names)

	return names
}
