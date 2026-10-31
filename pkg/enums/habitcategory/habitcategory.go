// Package habitcategory enumerates the allowed values of the habitcategory field.
package habitcategory

const (
	Morning   = "morning"
	Nutrition = "nutrition"
	Fitness   = "fitness"
	Hydration = "hydration"
	Sleep     = "sleep"
	Custom    = "custom"
)

func Values() []string { return []string{Morning, Nutrition, Fitness, Hydration, Sleep, Custom} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
