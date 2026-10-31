// Package targetcomparison enumerates the allowed values of the targetcomparison field.
package targetcomparison

const (
	AtLeast = "atLeast"
	AtMost  = "atMost"
	Between = "between"
	By      = "by"
)

func Values() []string { return []string{AtLeast, AtMost, Between, By} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
