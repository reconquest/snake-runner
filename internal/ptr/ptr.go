package ptr

import "time"

func TimePtr(value time.Time) *time.Time {
	return &value
}

func StringPtr(value string) *string {
	return &value
}
