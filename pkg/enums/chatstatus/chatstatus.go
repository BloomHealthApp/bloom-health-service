// Package chatstatus enumerates the allowed values of a chat message's status.
package chatstatus

const (
	Pending   = "pending"
	Running   = "running"
	Succeeded = "succeeded"
	Failed    = "failed"
)

func Values() []string { return []string{Pending, Running, Succeeded, Failed} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}

// Wire is the status clients see: a turn the worker is running is still pending to them.
func Wire(s string) string {
	if s == Running {
		return Pending
	}
	return s
}
