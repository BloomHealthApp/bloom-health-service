// Package opstatus enumerates the allowed values of the opstatus field.
package opstatus

const (
	Applied   = "applied"
	Duplicate = "duplicate"
	Conflict  = "conflict"
	Rejected  = "rejected"
)

func Values() []string { return []string{Applied, Duplicate, Conflict, Rejected} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
