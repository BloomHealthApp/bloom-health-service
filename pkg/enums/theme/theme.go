// Package theme enumerates the allowed values of the theme field.
package theme

const (
	ForestTrail   = "forestTrail"
	MidnightNavy  = "midnightNavy"
	CottonCandy   = "cottonCandy"
	LavenderDream = "lavenderDream"
	SunsetEmber   = "sunsetEmber"
	MintPeach     = "mintPeach"
)

func Values() []string {
	return []string{ForestTrail, MidnightNavy, CottonCandy, LavenderDream, SunsetEmber, MintPeach}
}

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
