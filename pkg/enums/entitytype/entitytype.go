// Package entitytype enumerates the allowed values of the entitytype field.
package entitytype

const (
	Habit    = "habit"
	DailyLog = "dailyLog"
	Workout  = "workout"
	Settings = "settings"
	Period   = "period"
)

// Values lists every allowed value.
func Values() []string { return []string{Habit, DailyLog, Workout, Settings, Period} }

// Valid reports whether s is an allowed value.
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
