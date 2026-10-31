// Package theme enumerates the allowed values of the theme field.
package theme

const (
	CottonCandy   = "cottonCandy"
	LavenderDream = "lavenderDream"
	MintPeach     = "mintPeach"
)

func Values() []string { return []string{CottonCandy, LavenderDream, MintPeach} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
