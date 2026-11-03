// Package logsource enumerates the allowed values of the source field on daily logs.
package logsource

const (
	Manual = "manual"
	Device = "device"
)

func Values() []string { return []string{Manual, Device} }
