// Package trackingtype enumerates the allowed values of the trackingtype field.
package trackingtype

const (
	Boolean   = "boolean"
	Quantity  = "quantity"
	Duration  = "duration"
	TimeOfDay = "timeOfDay"
)

func Values() []string { return []string{Boolean, Quantity, Duration, TimeOfDay} }
