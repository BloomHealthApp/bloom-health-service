// Package gender enumerates the allowed values of the gender profile field.
package gender

const (
	Female         = "female"
	Male           = "male"
	Other          = "other"
	PreferNotToSay = "prefer_not_to_say"
)

func Values() []string { return []string{Female, Male, Other, PreferNotToSay} }

// Valid reports whether s is an allowed value (exact, case-sensitive).
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
