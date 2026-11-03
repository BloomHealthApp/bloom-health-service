// Package chatstatus enumerates the allowed values of a chat message's status.
package chatstatus

const (
	Pending   = "pending"
	Running   = "running"
	Succeeded = "succeeded"
	Failed    = "failed"
)

// Wire is the status clients see: a turn the worker is running is still pending to them.
func Wire(s string) string {
	if s == Running {
		return Pending
	}
	return s
}
