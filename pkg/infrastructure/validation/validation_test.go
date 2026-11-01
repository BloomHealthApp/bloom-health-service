package validation

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
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

func TestLogSource(t *testing.T) {
	base := map[string]any{"habitId": "h", "localDate": "2026-10-18", "value": 1.0}

	d, err := Merge(entitytype.DailyLog, nil, base, Context{})
	require.NoError(t, err)
	require.Equal(t, "manual", d["source"])

	d, err = Merge(entitytype.DailyLog, nil, map[string]any{"habitId": "h", "localDate": "2026-10-18", "value": 1.0, "source": "device"}, Context{})
	require.NoError(t, err)
	require.Equal(t, "device", d["source"])

	d, err = Merge(entitytype.DailyLog, d, map[string]any{"value": 2.0}, Context{})
	require.NoError(t, err)
	require.Equal(t, "manual", d["source"], "a write without source must not keep a stored device value")

	_, err = Merge(entitytype.DailyLog, nil, map[string]any{"habitId": "h", "localDate": "2026-10-18", "source": "watch"}, Context{})
	require.Error(t, err)
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

func TestPeriodValidation(t *testing.T) {
	ctx := Context{Today: "2026-10-18"}
	d, err := Merge(entitytype.Period, nil, map[string]any{"startDate": "2026-10-10"}, ctx)
	require.NoError(t, err)
	require.Nil(t, d["endDate"])
	require.Equal(t, "medium", d["flow"])
	require.Equal(t, []any{}, d["symptoms"])
	require.Nil(t, d["mood"])
	require.Nil(t, d["note"])

	ok := map[string]any{"startDate": "2026-10-01", "endDate": "2026-10-14", "flow": "light", "mood": "happy", "symptoms": []any{"acne"}}
	_, err = Merge(entitytype.Period, nil, ok, ctx)
	require.NoError(t, err, "14 days inclusive is allowed")
	for name, p := range map[string]map[string]any{
		"future":   {"startDate": "2026-10-19"},
		"15 days":  {"startDate": "2026-10-01", "endDate": "2026-10-15"},
		"reversed": {"startDate": "2026-10-05", "endDate": "2026-10-04"},
		"flow":     {"startDate": "2026-10-01", "flow": "x"},
		"mood":     {"startDate": "2026-10-01", "mood": "x"},
		"symptom":  {"startDate": "2026-10-01", "symptoms": []any{"x"}},
		"notarray": {"startDate": "2026-10-01", "symptoms": "cramps"},
		"unknown":  {"startDate": "2026-10-01", "id2": "x"},
	} {
		_, err := Merge(entitytype.Period, nil, p, ctx)
		require.Error(t, err, name)
	}
	// ending a period keeps the other fields
	base, _ := Merge(entitytype.Period, nil, ok, ctx)
	d, err = Merge(entitytype.Period, base, map[string]any{"mood": nil}, ctx)
	require.NoError(t, err)
	require.Nil(t, d["mood"])
	require.Equal(t, "light", d["flow"])
}

func TestSettingsSundayRest(t *testing.T) {
	d, err := Merge(entitytype.Settings, nil, DefaultSettings(), Context{})
	require.NoError(t, err)
	require.Equal(t, true, d["sundayRest"])
	d, err = Merge(entitytype.Settings, d, map[string]any{"sundayRest": false}, Context{})
	require.NoError(t, err)
	require.Equal(t, false, d["sundayRest"])
	_, err = Merge(entitytype.Settings, d, map[string]any{"sundayRest": "no"}, Context{})
	require.Error(t, err)
}
