package main

const (
	StatusPending = "PENDING"
	StatusQueued  = "QUEUED"
	StatusRunning = "RUNNING"

	StatusSuccess  = "SUCCESS"
	StatusFailed   = "FAILED"
	StatusCanceled = "CANCELED"
	StatusSkipped  = "SKIPPED"

	StatusUnknown = "UNKNOWN"
)

// unused for now
func isFinalStatus(status string) bool {
	return status == StatusSuccess ||
		status == StatusFailed ||
		status == StatusCanceled ||
		status == StatusSkipped
}
