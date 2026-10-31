// Package workouttype enumerates the allowed values of the workouttype field.
package workouttype

const (
	Strength  = "strength"
	Cardio    = "cardio"
	Yoga      = "yoga"
	Walking   = "walking"
	Badminton = "badminton"
	Other     = "other"
)

func Values() []string { return []string{Strength, Cardio, Yoga, Walking, Badminton, Other} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
