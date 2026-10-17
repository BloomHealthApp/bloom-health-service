// Package targetcomparison enumerates the allowed values of the targetcomparison field.
package targetcomparison

const (
	AtLeast = "atLeast"
	AtMost  = "atMost"
	Between = "between"
	By      = "by"
)

// Values lists every allowed value.
func Values() []string { return []string{AtLeast, AtMost, Between, By} }

// Valid reports whether s is an allowed value.
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
