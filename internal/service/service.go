// Package service holds business logic shared by the REST and sync endpoints.
package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bubu-bloom/service/internal/entities"
	"github.com/bubu-bloom/service/internal/model"
	"github.com/bubu-bloom/service/internal/scoring"
	"github.com/bubu-bloom/service/internal/store"
)

const (
	StatusApplied   = "applied"
	StatusDuplicate = "duplicate"
	StatusConflict  = "conflict"
	StatusRejected  = "rejected"
	statusError     = "error"
)

type Service struct {
	Store store.Store
	Now   func() time.Time
}

func New(st store.Store) *Service { return &Service{Store: st, Now: time.Now} }

// Actor is the authenticated user with membership resolved server-side.
type Actor struct {
	User       *model.User
	Membership *model.Membership
}

func (a Actor) SpaceID() string { return a.Membership.SpaceID }
func (a Actor) IsOwner() bool   { return a.Membership.Role == model.RoleOwner }

type Op struct {
	OperationID     string
	EntityType      string
	EntityID        string
	Op              string // upsert | delete
	BaseVersion     int
	Payload         map[string]any
	ClientTimestamp *time.Time
}

type Result struct {
	OperationID string
	Status      string
	Reason      string
	Resolved    string
	Entity      *model.Entity
	Err         error // internal error (Status == "error")
}

func rejected(reason string) Result { return Result{Status: StatusRejected, Reason: reason} }

// LogID is the canonical id of a daily log.
func LogID(habitID, localDate string) string { return habitID + ":" + localDate }

func splitLogID(id string) (habitID, date string, ok bool) {
	i := strings.LastIndex(id, ":")
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}

func validEntityID(id string) bool {
	return id != "" && len(id) <= 128 && !strings.ContainsAny(id, "/\\ \t\r\n")
}

// Apply executes one write operation for an owner. opID may be empty for direct REST writes.
func (s *Service) Apply(ctx context.Context, a Actor, op Op) Result {
	if !a.IsOwner() {
		return rejected("forbidden")
	}
	if op.OperationID != "" {
		if rec, err := s.Store.GetOp(ctx, op.OperationID); err == nil {
			return replay(rec, a.SpaceID())
		}
	}
	res := s.apply(ctx, a, op)
	res.OperationID = op.OperationID
	if op.OperationID != "" && res.Status != statusError {
		rec := &model.OpRecord{OperationID: op.OperationID, SpaceID: a.SpaceID(), Status: res.Status,
			Reason: res.Reason, Resolved: res.Resolved, Entity: res.Entity, CreatedAt: s.Now().UTC()}
		if err := s.Store.SaveOp(ctx, rec); errors.Is(err, store.ErrConflict) {
			if prior, err := s.Store.GetOp(ctx, op.OperationID); err == nil {
				return replay(prior, a.SpaceID())
			}
		}
	}
	return res
}

func replay(rec *model.OpRecord, spaceID string) Result {
	if rec.SpaceID != spaceID {
		return Result{OperationID: rec.OperationID, Status: StatusRejected, Reason: "operation_id_in_use"}
	}
	st := rec.Status
	if st == StatusApplied {
		st = StatusDuplicate
	}
	return Result{OperationID: rec.OperationID, Status: st, Reason: rec.Reason, Resolved: rec.Resolved, Entity: rec.Entity}
}

func (s *Service) apply(ctx context.Context, a Actor, op Op) Result {
	known := false
	for _, t := range model.EntityTypes {
		known = known || t == op.EntityType
	}
	if !known {
		return rejected("unknown_entity_type")
	}
	if op.Op != "upsert" && op.Op != "delete" {
		return rejected("unknown_op")
	}
	if !validEntityID(op.EntityID) {
		return rejected("invalid_entity_id")
	}
	if op.EntityType == model.TypeSettings && (op.EntityID != model.SettingsID || op.Op == "delete") {
		return rejected("invalid_settings_operation")
	}
	space := a.SpaceID()
	now := s.Now().UTC()
	today := scoring.LocalDate(now, a.User.Timezone)
	payload := op.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	var habit *model.Entity
	if op.EntityType == model.TypeDailyLog && op.Op == "upsert" {
		hid, date, ok := splitLogID(op.EntityID)
		if !ok {
			return rejected("invalid_entity_id")
		}
		for k, want := range map[string]string{"habitId": hid, "localDate": date} {
			if v, present := payload[k]; present && v != want {
				return rejected("payload_id_mismatch")
			}
		}
		payload = copyMap(payload)
		payload["habitId"], payload["localDate"] = hid, date
		h, err := s.Store.GetEntity(ctx, space, model.TypeHabit, hid)
		if errors.Is(err, store.ErrNotFound) || (err == nil && h.DeletedAt != nil) {
			return rejected("unknown_habit")
		}
		if err != nil {
			return Result{Status: statusError, Err: err}
		}
		habit = h
	}

	existing, err := s.Store.GetEntity(ctx, space, op.EntityType, op.EntityID)
	exists := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return Result{Status: statusError, Err: err}
	}

	resolved := ""
	if exists && op.BaseVersion != existing.Version {
		lww := op.Op == "upsert" && op.EntityType == model.TypeDailyLog && op.ClientTimestamp != nil && op.ClientTimestamp.After(existing.UpdatedAt)
		if !lww {
			if op.Op == "delete" && existing.DeletedAt != nil {
				return Result{Status: StatusApplied, Entity: existing} // already deleted: idempotent
			}
			return Result{Status: StatusConflict, Entity: existing}
		}
		resolved = "lww"
	}

	var next *model.Entity
	switch op.Op {
	case "delete":
		if !exists {
			return rejected("not_found")
		}
		next = existing.Clone()
		next.DeletedAt = &now
	default:
		var base map[string]any
		if exists {
			base = existing.Data
		}
		if habit != nil {
			if _, hasStatus := payload["status"]; !hasStatus {
				var v *float64
				if f, ok := payload["value"].(float64); ok {
					v = &f
				}
				payload = copyMap(payload)
				payload["status"] = scoring.EvaluateStatus(HabitFromEntity(habit), v)
			}
		}
		data, err := entities.Merge(op.EntityType, base, payload, entities.Context{Today: today})
		if err != nil {
			var ve *entities.ValidationError
			if errors.As(err, &ve) {
				return rejected("validation_failed: " + ve.Msg)
			}
			return Result{Status: statusError, Err: err}
		}
		if exists {
			next = existing.Clone()
		} else {
			next = &model.Entity{ID: op.EntityID, Type: op.EntityType, SpaceID: space, CreatedAt: now}
		}
		next.Data = data
		next.DeletedAt = nil
	}
	if exists {
		next.Version = existing.Version + 1
	} else {
		next.Version = 1
	}
	next.UpdatedAt = now
	if err := s.Store.SaveEntity(ctx, next); err != nil {
		if errors.Is(err, store.ErrConflict) {
			if cur, gerr := s.Store.GetEntity(ctx, space, op.EntityType, op.EntityID); gerr == nil {
				return Result{Status: StatusConflict, Entity: cur}
			}
		}
		return Result{Status: statusError, Err: err}
	}
	return Result{Status: StatusApplied, Entity: next, Resolved: resolved}
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// HabitFromEntity converts a habit entity into scoring input.
func HabitFromEntity(e *model.Entity) scoring.Habit {
	h := scoring.Habit{ID: e.ID, Active: true}
	h.TrackingType, _ = e.Data["trackingType"].(string)
	h.Comparison, _ = e.Data["targetComparison"].(string)
	h.Target, _ = e.Data["target"].(float64)
	if v, ok := e.Data["targetMax"].(float64); ok {
		h.TargetMax = &v
	}
	if b, ok := e.Data["isActive"].(bool); ok {
		h.Active = b
	}
	if sch, ok := e.Data["schedule"].(map[string]any); ok {
		if days, ok := sch["days"].([]any); ok {
			for _, d := range days {
				if f, ok := d.(float64); ok {
					h.Days = append(h.Days, int(f))
				}
			}
		}
	}
	return h
}

// ---- pull ----

type Page struct {
	Cursor  string
	HasMore bool
	Changes map[string][]map[string]any
}

func EncodeCursor(seq int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("s:" + strconv.FormatInt(seq, 10)))
}

func DecodeCursor(c string) (int64, error) {
	if c == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || !strings.HasPrefix(string(b), "s:") {
		return 0, fmt.Errorf("invalid cursor")
	}
	n, err := strconv.ParseInt(string(b)[2:], 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return n, nil
}

var changeKeys = map[string]string{model.TypeHabit: "habits", model.TypeDailyLog: "dailyLogs", model.TypeWorkout: "workouts", model.TypeSettings: "settings"}

func (s *Service) Pull(ctx context.Context, spaceID, cursor string, limit int) (*Page, error) {
	after, err := DecodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := s.Store.Changes(ctx, spaceID, after, limit+1)
	if err != nil {
		return nil, err
	}
	page := &Page{Cursor: EncodeCursor(after), Changes: map[string][]map[string]any{"habits": {}, "dailyLogs": {}, "workouts": {}, "settings": {}}}
	if len(rows) > limit {
		page.HasMore = true
		rows = rows[:limit]
	}
	for i := range rows {
		k := changeKeys[rows[i].Type]
		page.Changes[k] = append(page.Changes[k], rows[i].Public())
		page.Cursor = EncodeCursor(rows[i].Seq)
	}
	return page, nil
}

// ---- stats ----

type Summary struct {
	CurrentStreak int           `json:"currentStreak"`
	LongestStreak int           `json:"longestStreak"`
	Days          []scoring.Day `json:"days"`
	WorkoutCount  int           `json:"workoutCount"`
	WorkoutSecs   int           `json:"workoutSeconds"`
	Milestones    []int         `json:"milestones"`
}

func (s *Service) OwnerTimezone(ctx context.Context, spaceID string) string {
	members, _ := s.Store.MembersOfSpace(ctx, spaceID)
	for _, m := range members {
		if m.Role == model.RoleOwner {
			if u, err := s.Store.UserByID(ctx, m.UserID); err == nil {
				return u.Timezone
			}
		}
	}
	return "UTC"
}

func (s *Service) Summary(ctx context.Context, spaceID, from, to string) (*Summary, error) {
	tz := s.OwnerTimezone(ctx, spaceID)
	today := scoring.LocalDate(s.Now(), tz)
	habitEnts, err := s.Store.ListEntities(ctx, spaceID, model.TypeHabit, store.ListQuery{})
	if err != nil {
		return nil, err
	}
	var habits []scoring.Habit
	for i := range habitEnts {
		habits = append(habits, HabitFromEntity(&habitEnts[i]))
	}
	logEnts, err := s.Store.ListEntities(ctx, spaceID, model.TypeDailyLog, store.ListQuery{})
	if err != nil {
		return nil, err
	}
	byDate := map[string]map[string]scoring.Log{}
	earliest := today
	for _, l := range logEnts {
		d, _ := l.Data["localDate"].(string)
		hid, _ := l.Data["habitId"].(string)
		if d == "" || hid == "" {
			continue
		}
		if byDate[d] == nil {
			byDate[d] = map[string]scoring.Log{}
		}
		lg := scoring.Log{}
		if v, ok := l.Data["value"].(float64); ok {
			lg.Value = &v
		}
		lg.Status, _ = l.Data["status"].(string)
		byDate[d][hid] = lg
		if d < earliest {
			earliest = d
		}
	}
	settings := scoring.Settings{Threshold: 80, RestMode: "neutral"}
	if se, err := s.Store.GetEntity(ctx, spaceID, model.TypeSettings, model.SettingsID); err == nil {
		if v, ok := se.Data["dailyThreshold"].(float64); ok {
			settings.Threshold = v
		}
		if v, ok := se.Data["restDayMode"].(string); ok {
			settings.RestMode = v
		}
		if arr, ok := se.Data["restDays"].([]any); ok {
			for _, x := range arr {
				if f, ok := x.(float64); ok {
					settings.RestDays = append(settings.RestDays, int(f))
				}
			}
		}
	}
	all, err := scoring.ComputeDays(habits, byDate, settings, earliest, today, today)
	if err != nil {
		return nil, err
	}
	cur, long := scoring.Streaks(all, today)
	var window []scoring.Day
	for _, d := range all {
		if d.Date >= from && d.Date <= to {
			window = append(window, d)
		}
	}
	if window == nil {
		window = []scoring.Day{}
	}
	sum := &Summary{CurrentStreak: cur, LongestStreak: long, Days: window, Milestones: scoring.MilestonesReached(long)}
	workouts, err := s.Store.ListEntities(ctx, spaceID, model.TypeWorkout, store.ListQuery{From: from, To: to})
	if err != nil {
		return nil, err
	}
	for _, w := range workouts {
		if _, ended := w.Data["endedAt"].(string); !ended {
			continue
		}
		sum.WorkoutCount++
		if d, ok := w.Data["durationSeconds"].(float64); ok {
			sum.WorkoutSecs += int(d)
		}
	}
	return sum, nil
}
