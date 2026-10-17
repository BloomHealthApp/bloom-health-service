// Package opstatus enumerates the allowed values of the opstatus field.
package opstatus

const (
	Applied   = "applied"
	Duplicate = "duplicate"
	Conflict  = "conflict"
	Rejected  = "rejected"
)

// Values lists every allowed value.
func Values() []string { return []string{Applied, Duplicate, Conflict, Rejected} }

// Valid reports whether s is an allowed value.
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
