// Package periodmood enumerates the allowed moods of a period entry.
package periodmood

const (
	Happy     = "happy"
	Calm      = "calm"
	Sad       = "sad"
	Irritable = "irritable"
	Anxious   = "anxious"
)

// Values lists every allowed value.
func Values() []string { return []string{Happy, Calm, Sad, Irritable, Anxious} }

// Valid reports whether s is an allowed value (exact, case-sensitive).
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
