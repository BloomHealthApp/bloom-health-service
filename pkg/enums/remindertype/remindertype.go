// Package remindertype enumerates the allowed values of the remindertype field.
package remindertype

const (
	WakeUp         = "wakeUp"
	Hydration      = "hydration"
	Workout        = "workout"
	EveningCheckIn = "eveningCheckIn"
	Bedtime        = "bedtime"
)

// Values lists every allowed value.
func Values() []string { return []string{WakeUp, Hydration, Workout, EveningCheckIn, Bedtime} }

// Valid reports whether s is an allowed value.
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
