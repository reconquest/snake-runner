package status

type Status string

const (
	PENDING  = Status("PENDING")
	QUEUED   = Status("QUEUED")
	RUNNING  = Status("RUNNING")
	SUCCESS  = Status("SUCCESS")
	FAILED   = Status("FAILED")
	CANCELED = Status("CANCELED")
	SKIPPED  = Status("SKIPPED")
	UNKNOWN  = Status("UNKNOWN")
)

// unused for now
func isFinalStatus(status Status) bool {
	return status == SUCCESS ||
		status == FAILED ||
		status == CANCELED ||
		status == SKIPPED
}
