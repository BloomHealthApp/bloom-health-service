// Package trackingtype enumerates the allowed values of the trackingtype field.
package trackingtype

const (
	Boolean   = "boolean"
	Quantity  = "quantity"
	Duration  = "duration"
	TimeOfDay = "timeOfDay"
)

// Values lists every allowed value.
func Values() []string { return []string{Boolean, Quantity, Duration, TimeOfDay} }

// Valid reports whether s is an allowed value.
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
