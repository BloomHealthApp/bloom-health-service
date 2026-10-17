// Package scoring implements habit status evaluation, daily completion and streaks.
// The same rules are mirrored in the mobile app and verified against testdata/scoring.json.
package scoring

import (
	"time"
)

const (
	Complete   = "complete"
	Incomplete = "incomplete"
	Skipped    = "skipped"
	Partial    = "partial"
)

var Milestones = []int{3, 7, 14, 30, 100}

type Habit struct {
	ID           string
	TrackingType string // boolean | quantity | duration | timeOfDay
	Target       float64
	TargetMax    *float64
	Comparison   string // atLeast | atMost | between | by
	Days         []int  // scheduled weekdays, 0=Sunday; empty = every day
	Active       bool
}

// Log is a day's entry for a habit. Status may be empty, in which case it is evaluated from Value.
type Log struct {
	Value  *float64
	Status string
}

type Settings struct {
	Threshold float64 // 0-100
	RestDays  []int
	RestMode  string // "neutral" (default) | "counts"
}

type Day struct {
	Date       string  `json:"localDate"`
	Percent    float64 `json:"completionPercent"`
	Successful bool    `json:"successful"`
	RestDay    bool    `json:"restDay"`
	Logged     bool    `json:"-"`
}

// EvaluateStatus derives a log status from a recorded value.
func EvaluateStatus(h Habit, v *float64) string {
	if v == nil {
		return Incomplete
	}
	val := *v
	switch h.TrackingType {
	case "boolean":
		if val >= 1 {
			return Complete
		}
		return Incomplete
	case "timeOfDay": // minutes since midnight, deadline semantics ("by")
		if val <= h.Target {
			return Complete
		}
		return Incomplete
	default: // quantity, duration
		if h.Comparison == "atMost" {
			if val <= h.Target {
				return Complete
			}
			return Incomplete
		}
		if val >= h.Target {
			return Complete
		}
		if val > 0 {
			return Partial
		}
		return Incomplete
	}
}

func scheduled(h Habit, weekday int) bool {
	if len(h.Days) == 0 {
		return true
	}
	for _, d := range h.Days {
		if d == weekday {
			return true
		}
	}
	return false
}

func statusOf(h Habit, l Log) string {
	if l.Status != "" {
		return l.Status
	}
	return EvaluateStatus(h, l.Value)
}

// DailyCompletion returns completed/eligible*100 (rounded to 2 decimals) and the eligible count.
// Eligible = active, scheduled that weekday, and not skipped. Partial counts as not completed.
func DailyCompletion(habits []Habit, logs map[string]Log, weekday int) (float64, int) {
	eligible, done := 0, 0
	for _, h := range habits {
		if !h.Active || !scheduled(h, weekday) {
			continue
		}
		l, ok := logs[h.ID]
		st := Incomplete
		if ok {
			st = statusOf(h, l)
		}
		if st == Skipped {
			continue
		}
		eligible++
		if st == Complete {
			done++
		}
	}
	if eligible == 0 {
		return 0, 0
	}
	p := float64(done) / float64(eligible) * 100
	return round2(p), eligible
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// ComputeDays evaluates every date in [from,to] that is not after today.
// logsByDate maps localDate -> habitID -> Log.
func ComputeDays(habits []Habit, logsByDate map[string]map[string]Log, s Settings, from, to, today string) ([]Day, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil, err
	}
	var out []Day
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		if ds > today {
			break
		}
		logs := logsByDate[ds]
		pct, _ := DailyCompletion(habits, logs, int(d.Weekday()))
		day := Day{Date: ds, Percent: pct, Logged: len(logs) > 0, RestDay: contains(s.RestDays, int(d.Weekday()))}
		day.Successful = day.Logged && pct >= s.Threshold && pct > 0
		if day.RestDay && s.RestMode == "counts" {
			day.Successful = true
		}
		out = append(out, day)
	}
	return out, nil
}

// Streaks returns current and longest streaks over chronologically ordered days.
// Rules: a successful day extends the run; an unsuccessful rest day is neutral (skipped);
// any other unsuccessful past day resets it; today, if not yet successful, neither extends nor resets.
func Streaks(days []Day, today string) (current, longest int) {
	run := 0
	for _, d := range days {
		switch {
		case d.Successful:
			run++
			if run > longest {
				longest = run
			}
		case d.RestDay:
			// neutral
		case d.Date == today:
			// still in progress
		default:
			run = 0
		}
	}
	return run, longest
}

func MilestonesReached(longest int) []int {
	out := []int{}
	for _, m := range Milestones {
		if longest >= m {
			out = append(out, m)
		}
	}
	return out
}

// LocalDate formats t as YYYY-MM-DD in the IANA timezone (falls back to UTC).
func LocalDate(t time.Time, tz string) string {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02")
}
