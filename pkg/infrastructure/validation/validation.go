// Package validation validates and normalizes entity payloads using strict allowlists,
// which also prevents mass-assignment of server-controlled fields.
package validation

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/habitcategory"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/logsource"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/logstatus"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/periodflow"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/periodmood"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/periodsymptom"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/remindertype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/restdaymode"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/targetcomparison"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/trackingtype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/workouttype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func bad(format string, a ...any) error { return &ValidationError{Msg: fmt.Sprintf(format, a...)} }

// Envelope keys the client may send but that the server controls.
var envelope = map[string]bool{"id": true, "version": true, "createdAt": true, "updatedAt": true, "deletedAt": true, "baseVersion": true}

var timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

var (
	categories   = set(habitcategory.Values()...)
	trackingTyps = set(trackingtype.Values()...)
	comparisons  = set(targetcomparison.Values()...)
	logStatuses  = set(logstatus.Values()...)
	logSources   = set(logsource.Values()...)
	workoutTyps  = set(workouttype.Values()...)
	restModes    = set(restdaymode.Values()...)
	reminderKeys = set(remindertype.Values()...)
	periodFlows  = set(periodflow.Values()...)
	periodMoods  = set(periodmood.Values()...)
	periodSymps  = set(periodsymptom.Values()...)
)

func set(v ...string) map[string]bool {
	m := map[string]bool{}
	for _, s := range v {
		m[s] = true
	}
	return m
}

var allowed = map[string]map[string]bool{
	entitytype.Habit: set("title", "icon", "category", "trackingType", "target", "targetMax", "targetComparison",
		"unit", "schedule", "reminder", "isActive", "sortOrder", "isDefault"),
	entitytype.DailyLog: set("habitId", "localDate", "value", "status", "note", "source"),
	entitytype.Workout:  set("type", "startedAt", "endedAt", "durationSeconds", "pausedSeconds", "notes", "localDate"),
	entitytype.Settings: set("dailyThreshold", "restDays", "restDayMode", "reminders", "stepGoal", "sundayRest"),
	entitytype.Period:   set("startDate", "endDate", "flow", "symptoms", "mood", "note"),
}

// serverOnly fields that clients may not set (silently kept from existing / defaulted on create).
var serverOnly = map[string]map[string]bool{entitytype.Habit: set("isDefault")}

// Context carries validation context.
type Context struct {
	Today string // writer's local date; daily logs/workouts after this are rejected
}

// Merge applies payload onto existing data (nil on create), validates the result and returns it.
func Merge(typ string, existing, payload map[string]any, ctx Context) (map[string]any, error) {
	allow, ok := allowed[typ]
	if !ok {
		return nil, bad("unknown entityType %q", typ)
	}
	out := map[string]any{}
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range payload {
		if envelope[k] {
			continue
		}
		if !allow[k] {
			return nil, bad("unknown field %q", k)
		}
		if serverOnly[typ][k] {
			continue
		}
		out[k] = v
	}
	var err error
	switch typ {
	case entitytype.Habit:
		err = validateHabit(out, existing == nil)
	case entitytype.DailyLog:
		// A writer that omits source (such as an older app) entered the value itself, so a stored "device" must not survive.
		if payload["source"] == nil {
			out["source"] = logsource.Manual
		}
		err = validateLog(out, ctx)
	case entitytype.Workout:
		err = validateWorkout(out, ctx)
	case entitytype.Settings:
		err = validateSettings(out)
	case entitytype.Period:
		err = validatePeriod(out, ctx)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func str(m map[string]any, k string, min, max int, required bool) (string, error) {
	v, present := m[k]
	if !present || v == nil {
		if required {
			return "", bad("%s is required", k)
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", bad("%s must be a string", k)
	}
	s = strings.TrimSpace(s)
	if len([]rune(s)) < min || len([]rune(s)) > max {
		return "", bad("%s must be %d-%d characters", k, min, max)
	}
	m[k] = s
	return s, nil
}

func num(m map[string]any, k string, required bool) (float64, bool, error) {
	v, present := m[k]
	if !present || v == nil {
		if required {
			return 0, false, bad("%s is required", k)
		}
		return 0, false, nil
	}
	switch n := v.(type) {
	case float64:
		return n, true, nil
	case int:
		m[k] = float64(n)
		return float64(n), true, nil
	case int64:
		m[k] = float64(n)
		return float64(n), true, nil
	}
	return 0, false, bad("%s must be a number", k)
}

func enum(m map[string]any, k string, allowedSet map[string]bool, required bool) error {
	s, err := str(m, k, 1, 40, required)
	if err != nil {
		return err
	}
	if s != "" && !allowedSet[s] {
		return bad("%s has invalid value %q", k, s)
	}
	return nil
}

func boolean(m map[string]any, k string, def bool) error {
	v, ok := m[k]
	if !ok || v == nil {
		m[k] = def
		return nil
	}
	if _, ok := v.(bool); !ok {
		return bad("%s must be a boolean", k)
	}
	return nil
}

func intList(v any, name string, lo, hi int) ([]any, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, bad("%s must be an array", name)
	}
	seen := map[int]bool{}
	out := make([]any, 0, len(arr))
	for _, x := range arr {
		f, ok := x.(float64)
		if !ok || f != float64(int(f)) || int(f) < lo || int(f) > hi {
			return nil, bad("%s entries must be integers %d-%d", name, lo, hi)
		}
		if !seen[int(f)] {
			seen[int(f)] = true
			out = append(out, f)
		}
	}
	return out, nil
}

func validateHabit(m map[string]any, create bool) error {
	if _, err := str(m, "title", 1, 80, true); err != nil {
		return err
	}
	if _, err := str(m, "icon", 0, 32, false); err != nil {
		return err
	}
	if m["icon"] == nil {
		m["icon"] = "sparkles"
	}
	if m["category"] == nil {
		m["category"] = "custom"
	}
	if err := enum(m, "category", categories, true); err != nil {
		return err
	}
	if err := enum(m, "trackingType", trackingTyps, true); err != nil {
		return err
	}
	if m["targetComparison"] == nil {
		m["targetComparison"] = map[bool]string{true: "by", false: "atLeast"}[m["trackingType"] == "timeOfDay"]
	}
	if err := enum(m, "targetComparison", comparisons, true); err != nil {
		return err
	}
	t, _, err := num(m, "target", m["trackingType"] != "boolean")
	if err != nil {
		return err
	}
	if m["target"] == nil {
		m["target"] = float64(1)
		t = 1
	}
	if t < 0 || t > 100000 {
		return bad("target out of range")
	}
	if tm, has, err := num(m, "targetMax", false); err != nil {
		return err
	} else if has && tm < t {
		return bad("targetMax must be >= target")
	} else if !has {
		m["targetMax"] = nil
	}
	if m["trackingType"] == "timeOfDay" && t >= 1440 {
		return bad("timeOfDay target must be minutes since midnight (<1440)")
	}
	if _, err := str(m, "unit", 0, 16, false); err != nil {
		return err
	}
	if m["unit"] == nil || m["unit"] == "" {
		m["unit"] = nil
	}
	// schedule
	sch, ok := m["schedule"].(map[string]any)
	if m["schedule"] == nil {
		sch, ok = map[string]any{"days": []any{}}, true
	}
	if !ok {
		return bad("schedule must be an object")
	}
	for k := range sch {
		if k != "days" {
			return bad("unknown field schedule.%s", k)
		}
	}
	days, err := intList(orDefault(sch["days"], []any{}), "schedule.days", 0, 6)
	if err != nil {
		return err
	}
	m["schedule"] = map[string]any{"days": days}
	// reminder
	if r := m["reminder"]; r != nil {
		rm, ok := r.(map[string]any)
		if !ok {
			return bad("reminder must be an object or null")
		}
		clean, err := cleanReminder(rm, false)
		if err != nil {
			return err
		}
		m["reminder"] = clean
	} else {
		m["reminder"] = nil
	}
	if err := boolean(m, "isActive", true); err != nil {
		return err
	}
	if _, _, err := num(m, "sortOrder", false); err != nil {
		return err
	}
	if m["sortOrder"] == nil {
		m["sortOrder"] = float64(0)
	}
	if create {
		m["isDefault"] = false
	} else if m["isDefault"] == nil {
		m["isDefault"] = false
	}
	return nil
}

func orDefault(v, d any) any {
	if v == nil {
		return d
	}
	return v
}

func cleanReminder(rm map[string]any, withKey bool) (map[string]any, error) {
	out := map[string]any{}
	for k := range rm {
		if k != "enabled" && k != "time" && !(withKey && k == "key") {
			return nil, bad("unknown field reminder.%s", k)
		}
	}
	en, ok := rm["enabled"].(bool)
	if !ok {
		return nil, bad("reminder.enabled must be a boolean")
	}
	tm, ok := rm["time"].(string)
	if !ok || !timeRe.MatchString(tm) {
		return nil, bad("reminder.time must be HH:mm")
	}
	out["enabled"], out["time"] = en, tm
	if withKey {
		k, _ := rm["key"].(string)
		if !reminderKeys[k] {
			return nil, bad("reminder.key invalid")
		}
		out["key"] = k
	}
	return out, nil
}

// ParseDate validates YYYY-MM-DD and reports failures as ValidationError.
func ParseDate(s string) (time.Time, error) {
	t, err := utils.ParseDate(s)
	if err != nil {
		return time.Time{}, bad("%s", err.Error())
	}
	return t, nil
}

func validateLog(m map[string]any, ctx Context) error {
	if _, err := str(m, "habitId", 1, 100, true); err != nil {
		return err
	}
	d, err := str(m, "localDate", 10, 10, true)
	if err != nil {
		return err
	}
	if _, err := ParseDate(d); err != nil {
		return err
	}
	if ctx.Today != "" && d > ctx.Today {
		return bad("localDate cannot be in the future")
	}
	if v, has, err := num(m, "value", false); err != nil {
		return err
	} else if has && (v < 0 || v > 1e6) {
		return bad("value out of range")
	} else if !has {
		m["value"] = nil
	}
	if m["status"] != nil {
		if err := enum(m, "status", logStatuses, false); err != nil {
			return err
		}
	}
	if _, err := str(m, "note", 0, 500, false); err != nil {
		return err
	}
	if m["note"] == nil || m["note"] == "" {
		m["note"] = nil
	}
	return enum(m, "source", logSources, true)
}

func validateWorkout(m map[string]any, ctx Context) error {
	if err := enum(m, "type", workoutTyps, true); err != nil {
		return err
	}
	sa, err := str(m, "startedAt", 20, 40, true)
	if err != nil {
		return err
	}
	start, err := time.Parse(time.RFC3339, sa)
	if err != nil {
		return bad("startedAt must be RFC3339")
	}
	m["startedAt"] = start.UTC().Format(time.RFC3339)
	paused, has, err := num(m, "pausedSeconds", false)
	if err != nil {
		return err
	}
	if !has {
		m["pausedSeconds"] = float64(0)
	} else if paused < 0 {
		return bad("pausedSeconds must be >= 0")
	}
	if ea, _ := m["endedAt"].(string); ea != "" {
		end, err := time.Parse(time.RFC3339, ea)
		if err != nil {
			return bad("endedAt must be RFC3339")
		}
		if end.Before(start) {
			return bad("endedAt must not be before startedAt")
		}
		dur := end.Sub(start).Seconds() - paused
		if dur < 0 {
			return bad("pausedSeconds exceeds workout length")
		}
		m["endedAt"] = end.UTC().Format(time.RFC3339)
		m["durationSeconds"] = float64(int64(dur))
	} else {
		if m["endedAt"] != nil && m["endedAt"] != "" {
			return bad("endedAt must be a string or null")
		}
		m["endedAt"] = nil
		if d, has, err := num(m, "durationSeconds", false); err != nil {
			return err
		} else if !has {
			m["durationSeconds"] = float64(0)
		} else if d < 0 {
			return bad("durationSeconds must be >= 0")
		}
	}
	if _, err := str(m, "notes", 0, 1000, false); err != nil {
		return err
	}
	if m["notes"] == nil {
		m["notes"] = ""
	}
	if m["localDate"] == nil {
		m["localDate"] = start.UTC().Format("2006-01-02")
	}
	d, err := str(m, "localDate", 10, 10, true)
	if err != nil {
		return err
	}
	if _, err := ParseDate(d); err != nil {
		return err
	}
	if ctx.Today != "" && d > ctx.Today {
		return bad("localDate cannot be in the future")
	}
	return nil
}

// DefaultReminders are the suggested defaults from the product spec.
func DefaultReminders() []any {
	mk := func(k, t string, on bool) any { return map[string]any{"key": k, "enabled": on, "time": t} }
	return []any{mk("wakeUp", "05:00", true), mk("hydration", "11:00", true), mk("workout", "17:00", true),
		mk("eveningCheckIn", "21:30", true), mk("bedtime", "21:30", true)}
}

// DefaultSettings returns the initial settings data.
func DefaultSettings() map[string]any {
	return map[string]any{"dailyThreshold": float64(80), "restDays": []any{}, "restDayMode": "neutral",
		"reminders": DefaultReminders(), "stepGoal": float64(7000), "sundayRest": true}
}

func validateSettings(m map[string]any) error {
	th, has, err := num(m, "dailyThreshold", false)
	if err != nil {
		return err
	}
	if !has {
		m["dailyThreshold"] = float64(80)
	} else if th < 0 || th > 100 {
		return bad("dailyThreshold must be 0-100")
	}
	rd, err := intList(orDefault(m["restDays"], []any{}), "restDays", 0, 6)
	if err != nil {
		return err
	}
	m["restDays"] = rd
	if m["restDayMode"] == nil {
		m["restDayMode"] = "neutral"
	}
	if err := enum(m, "restDayMode", restModes, true); err != nil {
		return err
	}
	if sg, has, err := num(m, "stepGoal", false); err != nil {
		return err
	} else if !has {
		m["stepGoal"] = float64(7000)
	} else if sg < 0 || sg > 100000 {
		return bad("stepGoal out of range")
	}
	if err := boolean(m, "sundayRest", true); err != nil {
		return err
	}
	if m["reminders"] == nil {
		m["reminders"] = DefaultReminders()
		return nil
	}
	arr, ok := m["reminders"].([]any)
	if !ok || len(arr) > 10 {
		return bad("reminders must be an array")
	}
	seen := map[string]bool{}
	clean := make([]any, 0, len(arr))
	for _, r := range arr {
		rm, ok := r.(map[string]any)
		if !ok {
			return bad("reminders entries must be objects")
		}
		c, err := cleanReminder(rm, true)
		if err != nil {
			return err
		}
		k := c["key"].(string)
		if seen[k] {
			return bad("duplicate reminder %q", k)
		}
		seen[k] = true
		clean = append(clean, c)
	}
	m["reminders"] = clean
	return nil
}

// MaxPeriodDays is the maximum length of a period in days (inclusive of both ends).
const MaxPeriodDays = 14

// MaxPeriodSymptoms is the maximum number of symptoms on one entry.
const MaxPeriodSymptoms = 10

// PeriodRange returns the start and end dates of a validated period entity data map; end is "" while ongoing.
func PeriodRange(data map[string]any) (start, end string) {
	start, _ = data["startDate"].(string)
	end, _ = data["endDate"].(string)
	return start, end
}

func validatePeriod(m map[string]any, ctx Context) error {
	start, err := str(m, "startDate", 10, 10, true)
	if err != nil {
		return err
	}
	st, err := ParseDate(start)
	if err != nil {
		return err
	}
	if ctx.Today != "" && start > ctx.Today {
		return bad("startDate cannot be in the future")
	}
	if m["endDate"] == nil || m["endDate"] == "" {
		m["endDate"] = nil
	} else {
		end, err := str(m, "endDate", 10, 10, true)
		if err != nil {
			return err
		}
		et, err := ParseDate(end)
		if err != nil {
			return err
		}
		if et.Before(st) {
			return bad("endDate must not be before startDate")
		}
		if int(et.Sub(st).Hours()/24)+1 > MaxPeriodDays {
			return bad("a period can last at most %d days", MaxPeriodDays)
		}
	}
	if m["flow"] == nil {
		m["flow"] = periodflow.Medium
	}
	if err := enum(m, "flow", periodFlows, true); err != nil {
		return err
	}
	if m["mood"] == nil || m["mood"] == "" {
		m["mood"] = nil
	} else if err := enum(m, "mood", periodMoods, false); err != nil {
		return err
	}
	if m["symptoms"] == nil {
		m["symptoms"] = []any{}
	} else {
		arr, ok := m["symptoms"].([]any)
		if !ok {
			return bad("symptoms must be an array")
		}
		if len(arr) > MaxPeriodSymptoms {
			return bad("at most %d symptoms", MaxPeriodSymptoms)
		}
		seen := map[string]bool{}
		clean := make([]any, 0, len(arr))
		for _, x := range arr {
			sy, ok := x.(string)
			if !ok || !periodSymps[sy] {
				return bad("symptoms has an invalid entry")
			}
			if !seen[sy] {
				seen[sy] = true
				clean = append(clean, sy)
			}
		}
		m["symptoms"] = clean
	}
	if _, err := str(m, "note", 0, 500, false); err != nil {
		return err
	}
	if m["note"] == nil || m["note"] == "" {
		m["note"] = nil
	}
	return nil
}
