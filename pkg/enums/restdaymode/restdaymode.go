// Package restdaymode enumerates the allowed values of the restdaymode field.
package restdaymode

const (
	Neutral = "neutral"
	Counts  = "counts"
)

func Values() []string { return []string{Neutral, Counts} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
