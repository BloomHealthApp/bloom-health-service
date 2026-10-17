package scoring

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fixtureHabit struct {
	ID           string   `json:"id"`
	TrackingType string   `json:"trackingType"`
	Target       float64  `json:"target"`
	Comparison   string   `json:"comparison"`
	Days         []int    `json:"days"`
	Active       bool     `json:"active"`
	TargetMax    *float64 `json:"targetMax"`
}

type fixtureLog struct {
	Value  *float64 `json:"value"`
	Status string   `json:"status"`
}

type fixture struct {
	Habits []fixtureHabit `json:"habits"`
	Cases  []struct {
		Name     string `json:"name"`
		Settings struct {
			Threshold float64 `json:"threshold"`
			RestDays  []int   `json:"restDays"`
			RestMode  string  `json:"restMode"`
		} `json:"settings"`
		From   string                           `json:"from"`
		To     string                           `json:"to"`
		Today  string                           `json:"today"`
		Logs   map[string]map[string]fixtureLog `json:"logs"`
		Expect struct {
			Current  int                `json:"current"`
			Longest  int                `json:"longest"`
			Percents map[string]float64 `json:"percents"`
		} `json:"expect"`
	} `json:"cases"`
}

func TestFixtures(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/scoring.json")
	require.NoError(t, err)
	var fx fixture
	require.NoError(t, json.Unmarshal(raw, &fx))
	var habits []Habit
	for _, h := range fx.Habits {
		habits = append(habits, Habit{ID: h.ID, TrackingType: h.TrackingType, Target: h.Target, TargetMax: h.TargetMax, Comparison: h.Comparison, Days: h.Days, Active: h.Active})
	}
	for _, c := range fx.Cases {
		t.Run(c.Name, func(t *testing.T) {
			logs := map[string]map[string]Log{}
			for d, m := range c.Logs {
				logs[d] = map[string]Log{}
				for id, l := range m {
					logs[d][id] = Log{Value: l.Value, Status: l.Status}
				}
			}
			days, err := ComputeDays(habits, logs, Settings{Threshold: c.Settings.Threshold, RestDays: c.Settings.RestDays, RestMode: c.Settings.RestMode}, c.From, c.To, c.Today)
			require.NoError(t, err)
			cur, long := Streaks(days, c.Today)
			require.Equal(t, c.Expect.Current, cur, "current")
			require.Equal(t, c.Expect.Longest, long, "longest")
			for _, d := range days {
				if want, ok := c.Expect.Percents[d.Date]; ok {
					require.InDelta(t, want, d.Percent, 0.001, d.Date)
				}
			}
		})
	}
}

func TestEvaluateStatus(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	require.Equal(t, Complete, EvaluateStatus(Habit{TrackingType: "boolean"}, f(1)))
	require.Equal(t, Incomplete, EvaluateStatus(Habit{TrackingType: "boolean"}, nil))
	require.Equal(t, Partial, EvaluateStatus(Habit{TrackingType: "quantity", Target: 2000, Comparison: "atLeast"}, f(1000)))
	require.Equal(t, Complete, EvaluateStatus(Habit{TrackingType: "duration", Target: 90, Comparison: "between"}, f(130)))
	require.Equal(t, Complete, EvaluateStatus(Habit{TrackingType: "timeOfDay", Target: 300, Comparison: "by"}, f(300)))
	require.Equal(t, Incomplete, EvaluateStatus(Habit{TrackingType: "timeOfDay", Target: 300, Comparison: "by"}, f(301)))
	require.Equal(t, Incomplete, EvaluateStatus(Habit{TrackingType: "quantity", Target: 5, Comparison: "atMost"}, f(6)))
}

func TestLocalDateTimezoneBoundary(t *testing.T) {
	ts := time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC) // 01:30 next day in Kolkata
	require.Equal(t, "2026-10-17", LocalDate(ts, "UTC"))
	require.Equal(t, "2026-10-18", LocalDate(ts, "Asia/Kolkata"))
	require.Equal(t, "2026-10-17", LocalDate(ts, "America/New_York"))
	require.Equal(t, "2026-10-17", LocalDate(ts, "Not/AZone"))
}

func TestMilestones(t *testing.T) {
	require.Equal(t, []int{3, 7}, MilestonesReached(10))
	require.Equal(t, []int{}, MilestonesReached(2))
}
