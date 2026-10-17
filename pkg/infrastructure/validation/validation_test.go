package validation

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bubu-bloom/service/pkg/enums/entitytype"
)

func TestHabitRejectsUnknownAndServerFields(t *testing.T) {
	_, err := Merge(entitytype.Habit, nil, map[string]any{"title": "x", "trackingType": "boolean", "sharedSpaceId": "evil"}, Context{})
	require.Error(t, err)
	_, err = Merge(entitytype.Habit, nil, map[string]any{"title": "x", "trackingType": "boolean", "role": "owner"}, Context{})
	require.Error(t, err)
}

func TestHabitDefaultsAndIsDefaultProtected(t *testing.T) {
	d, err := Merge(entitytype.Habit, nil, map[string]any{"title": " Stretch ", "trackingType": "boolean", "isDefault": true}, Context{})
	require.NoError(t, err)
	require.Equal(t, "Stretch", d["title"])
	require.Equal(t, false, d["isDefault"])
	require.Equal(t, true, d["isActive"])
	require.Equal(t, "custom", d["category"])
}

func TestLogFutureDateRejected(t *testing.T) {
	_, err := Merge(entitytype.DailyLog, nil, map[string]any{"habitId": "h", "localDate": "2026-10-19", "value": 1.0}, Context{Today: "2026-10-18"})
	require.Error(t, err)
	_, err = Merge(entitytype.DailyLog, nil, map[string]any{"habitId": "h", "localDate": "2026-10-18", "value": 1.0}, Context{Today: "2026-10-18"})
	require.NoError(t, err)
}

func TestWorkoutDurationDerivedFromTimestamps(t *testing.T) {
	d, err := Merge(entitytype.Workout, nil, map[string]any{"type": "yoga", "startedAt": "2026-10-17T05:00:00Z", "endedAt": "2026-10-17T06:00:00Z",
		"pausedSeconds": 300.0, "durationSeconds": 1.0, "localDate": "2026-10-17"}, Context{Today: "2026-10-17"})
	require.NoError(t, err)
	require.Equal(t, 3300.0, d["durationSeconds"])
	_, err = Merge(entitytype.Workout, nil, map[string]any{"type": "yoga", "startedAt": "2026-10-17T05:00:00Z", "endedAt": "2026-10-17T04:00:00Z"}, Context{})
	require.Error(t, err)
}

func TestSettingsValidation(t *testing.T) {
	_, err := Merge(entitytype.Settings, nil, map[string]any{"dailyThreshold": 120.0}, Context{})
	require.Error(t, err)
	d, err := Merge(entitytype.Settings, nil, map[string]any{"restDays": []any{0.0, 0.0, 6.0}}, Context{})
	require.NoError(t, err)
	require.Len(t, d["restDays"], 2)
	_, err = Merge(entitytype.Settings, nil, map[string]any{"reminders": []any{map[string]any{"key": "bedtime", "enabled": true, "time": "25:00"}}}, Context{})
	require.Error(t, err)
}
