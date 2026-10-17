// Package restdaymode enumerates the allowed values of the restdaymode field.
package restdaymode

const (
	Neutral = "neutral"
	Counts  = "counts"
)

// Values lists every allowed value.
func Values() []string { return []string{Neutral, Counts} }

// Valid reports whether s is an allowed value.
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
