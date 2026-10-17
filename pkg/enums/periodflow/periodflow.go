// Package periodflow enumerates the allowed flow levels of a period entry.
package periodflow

const (
	Light  = "light"
	Medium = "medium"
	Heavy  = "heavy"
)

// Values lists every allowed value.
func Values() []string { return []string{Light, Medium, Heavy} }

// Valid reports whether s is an allowed value (exact, case-sensitive).
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
