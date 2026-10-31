// Package logstatus enumerates the allowed values of the logstatus field.
package logstatus

const (
	Complete   = "complete"
	Incomplete = "incomplete"
	Skipped    = "skipped"
	Partial    = "partial"
)

func Values() []string { return []string{Complete, Incomplete, Skipped, Partial} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
