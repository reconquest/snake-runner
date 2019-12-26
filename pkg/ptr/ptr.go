package ptr

import "time"

func TimePtr(value time.Time) *time.Time {
	return &value
}
