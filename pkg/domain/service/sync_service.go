package service

import (
	"context"
	"strings"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/opstatus"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/validation"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/scoring"
)

// ISyncService is the single write path for every entity (REST writes and offline sync both use it):
// authorization, operationId idempotency, validation, compare-and-swap on version and conflict
// resolution live here.
type ISyncService interface {
	// Apply executes one write for an owner. A non-owner yields a rejected "forbidden" result.
	// Business outcomes (conflict, rejected) are results; only infrastructure failures are errors.
	Apply(ctx context.Context, actor *serviceBos.Actor, op serviceBos.ApplyOp) (*serviceBos.ApplyResult, apperrors.IHttpError)
	Push(ctx context.Context, actor *serviceBos.Actor, req *request.SyncPushReq) (*response.SyncPushResponse, apperrors.IHttpError)
	Pull(ctx context.Context, actor *serviceBos.Actor, req *request.SyncPullReq) (*response.SyncPullResponse, apperrors.IHttpError)
}

type syncService struct {
	log      logger.ILogger
	entities mongorepo.IEntityRepo
	ops      mongorepo.IOpRepo
	clock    clock.Clock
}

// NewSyncService wires the sync service.
func NewSyncService(log logger.ILogger, entities mongorepo.IEntityRepo, ops mongorepo.IOpRepo, clk clock.Clock) ISyncService {
	return &syncService{log: log, entities: entities, ops: ops, clock: clk}
}

// ReasonPeriodTrackerDisabled is the rejection reason for period writes while the owner's tracker is off.
const ReasonPeriodTrackerDisabled = "period_tracker_disabled"

const (
	opUpsert = "upsert"
	opDelete = "delete"
)

func rejected(reason string) *serviceBos.ApplyResult {
	return &serviceBos.ApplyResult{Status: opstatus.Rejected, Reason: reason}
}

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

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (s *syncService) Apply(ctx context.Context, a *serviceBos.Actor, op serviceBos.ApplyOp) (*serviceBos.ApplyResult, apperrors.IHttpError) {
	if !a.IsOwner() {
		return rejected("forbidden"), nil
	}
	if op.EntityType == entitytype.Period && !a.User.IsPeriodTrackerEnabled() {
		return rejected(ReasonPeriodTrackerDisabled), nil
	}
	if op.OperationID != "" {
		if rec, err := s.ops.Get(ctx, op.OperationID); err == nil {
			return replay(rec, a.SpaceID()), nil
		}
	}
	res, err := s.apply(ctx, a, op)
	if err != nil {
		return nil, internalErr(s.log, ctx, "apply failed", err)
	}
	res.OperationID = op.OperationID
	if op.OperationID != "" {
		rec := &dao.OpRecord{OperationID: op.OperationID, SpaceID: a.SpaceID(), Status: res.Status,
			Reason: res.Reason, Resolved: res.Resolved, Entity: res.Entity, CreatedAt: s.clock.Now().UTC()}
		if err := s.ops.Save(ctx, rec); apperrors.IsConflict(err) {
			if prior, err := s.ops.Get(ctx, op.OperationID); err == nil {
				return replay(prior, a.SpaceID()), nil
			}
		}
	}
	return res, nil
}

func replay(rec *dao.OpRecord, spaceID string) *serviceBos.ApplyResult {
	if rec.SpaceID != spaceID {
		return &serviceBos.ApplyResult{OperationID: rec.OperationID, Status: opstatus.Rejected, Reason: "operation_id_in_use"}
	}
	st := rec.Status
	if st == opstatus.Applied {
		st = opstatus.Duplicate
	}
	return &serviceBos.ApplyResult{OperationID: rec.OperationID, Status: st, Reason: rec.Reason, Resolved: rec.Resolved, Entity: rec.Entity}
}

// apply returns an error only for infrastructure failures.
func (s *syncService) apply(ctx context.Context, a *serviceBos.Actor, op serviceBos.ApplyOp) (*serviceBos.ApplyResult, error) {
	if !entitytype.Valid(op.EntityType) {
		return rejected("unknown_entity_type"), nil
	}
	if op.Op != opUpsert && op.Op != opDelete {
		return rejected("unknown_op"), nil
	}
	if !validEntityID(op.EntityID) {
		return rejected("invalid_entity_id"), nil
	}
	if op.EntityType == entitytype.Settings && (op.EntityID != constants.SettingsID || op.Op == opDelete) {
		return rejected("invalid_settings_operation"), nil
	}
	space := a.SpaceID()
	now := s.clock.Now().UTC()
	today := scoring.LocalDate(now, a.User.Timezone)
	payload := op.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	var habit *dao.Entity
	if op.EntityType == entitytype.DailyLog && op.Op == opUpsert {
		hid, date, ok := splitLogID(op.EntityID)
		if !ok {
			return rejected("invalid_entity_id"), nil
		}
		for k, want := range map[string]string{"habitId": hid, "localDate": date} {
			if v, present := payload[k]; present && v != want {
				return rejected("payload_id_mismatch"), nil
			}
		}
		payload = copyMap(payload)
		payload["habitId"], payload["localDate"] = hid, date
		h, err := s.entities.Get(ctx, space, entitytype.Habit, hid)
		if apperrors.IsNotFound(err) || (err == nil && h.DeletedAt != nil) {
			return rejected("unknown_habit"), nil
		}
		if err != nil {
			return nil, err
		}
		habit = h
	}

	existing, err := s.entities.Get(ctx, space, op.EntityType, op.EntityID)
	exists := err == nil
	if err != nil && !apperrors.IsNotFound(err) {
		return nil, err
	}

	resolved := ""
	if exists && op.BaseVersion != existing.Version {
		lww := op.Op == opUpsert && op.EntityType == entitytype.DailyLog && op.ClientTimestamp != nil && op.ClientTimestamp.After(existing.UpdatedAt)
		if !lww {
			if op.Op == opDelete && existing.DeletedAt != nil {
				return &serviceBos.ApplyResult{Status: opstatus.Applied, Entity: existing}, nil // already deleted: idempotent
			}
			return &serviceBos.ApplyResult{Status: opstatus.Conflict, Entity: existing}, nil
		}
		resolved = "lww"
	}

	var next *dao.Entity
	switch op.Op {
	case opDelete:
		if !exists {
			return rejected("not_found"), nil
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
				payload["status"] = scoring.EvaluateStatus(habitFromEntity(habit), v)
			}
		}
		data, err := validation.Merge(op.EntityType, base, payload, validation.Context{Today: today})
		if err != nil {
			if ve, ok := err.(*validation.ValidationError); ok {
				return rejected("validation_failed: " + ve.Msg), nil
			}
			return nil, err
		}
		if op.EntityType == entitytype.Period {
			if reason, err := s.checkPeriodRules(ctx, space, op.EntityID, data, today); err != nil {
				return nil, err
			} else if reason != "" {
				return rejected(reason), nil
			}
		}
		if exists {
			next = existing.Clone()
		} else {
			next = &dao.Entity{ID: op.EntityID, Type: op.EntityType, SpaceID: space, CreatedAt: now}
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
	if err := s.entities.Save(ctx, next); err != nil {
		if apperrors.IsConflict(err) {
			if cur, gerr := s.entities.Get(ctx, space, op.EntityType, op.EntityID); gerr == nil {
				return &serviceBos.ApplyResult{Status: opstatus.Conflict, Entity: cur}, nil
			}
		}
		return nil, err
	}
	return &serviceBos.ApplyResult{Status: opstatus.Applied, Entity: next, Resolved: resolved}, nil
}

func (s *syncService) Push(ctx context.Context, a *serviceBos.Actor, req *request.SyncPushReq) (*response.SyncPushResponse, apperrors.IHttpError) {
	if !a.IsOwner() {
		return nil, apperrors.ForbiddenError("read-only access")
	}
	results := make([]response.SyncResult, 0, len(req.Operations))
	for _, o := range req.Operations {
		if o.OperationID == "" || len(o.OperationID) > 64 {
			results = append(results, response.SyncResult{OperationID: o.OperationID, Status: opstatus.Rejected, Reason: "invalid_operation_id"})
			continue
		}
		res, herr := s.Apply(ctx, a, serviceBos.ApplyOp{OperationID: o.OperationID, EntityType: o.EntityType, EntityID: o.EntityID,
			Op: o.Op, BaseVersion: o.BaseVersion, Payload: o.Payload, ClientTimestamp: o.ClientTimestamp})
		if herr != nil {
			return nil, herr
		}
		item := response.SyncResult{OperationID: o.OperationID, Status: res.Status, Reason: res.Reason, Resolved: res.Resolved}
		if res.Entity != nil {
			item.Entity = res.Entity.Public()
			item.EntityType = res.Entity.Type
		}
		results = append(results, item)
	}
	return &response.SyncPushResponse{Results: results}, nil
}

var changeKeys = map[string]string{entitytype.Habit: "habits", entitytype.DailyLog: "dailyLogs", entitytype.Workout: "workouts",
	entitytype.Settings: "settings", entitytype.Period: "periods"}

// checkPeriodRules enforces the cross-record period rules: a range must not overlap another non-deleted
// period and only one period may be ongoing. An ongoing period is treated as lasting through today.
// It returns a rejection reason ("" when fine); the error is for infrastructure failures only.
func (s *syncService) checkPeriodRules(ctx context.Context, space, id string, data map[string]any, today string) (string, error) {
	start, end := validation.PeriodRange(data)
	ongoing := end == ""
	effEnd := end
	if ongoing {
		effEnd = maxStr(today, start)
	}
	others, err := s.entities.List(ctx, space, entitytype.Period, mongorepo.EntityQuery{})
	if err != nil {
		return "", err
	}
	for i := range others {
		if others[i].ID == id {
			continue
		}
		os, oe := validation.PeriodRange(others[i].Data)
		if ongoing && oe == "" {
			return "validation_failed: only one period can be ongoing", nil
		}
		oEff := oe
		if oe == "" {
			oEff = maxStr(today, os)
		}
		if start <= oEff && os <= effEnd {
			return "validation_failed: periods must not overlap", nil
		}
	}
	return "", nil
}

func maxStr(a, b string) string {
	if a > b {
		return a
	}
	return b
}

func (s *syncService) Pull(ctx context.Context, a *serviceBos.Actor, req *request.SyncPullReq) (*response.SyncPullResponse, apperrors.IHttpError) {
	limit := req.Limit
	if limit <= 0 || limit > constants.MaxPullLimit {
		limit = constants.DefaultPullLimit
	}
	rows, err := s.entities.Changes(ctx, a.SpaceID(), req.After, limit+1)
	if err != nil {
		return nil, internalErr(s.log, ctx, "pull failed", err)
	}
	out := &response.SyncPullResponse{Cursor: utils.EncodeCursor(req.After),
		Changes: map[string][]map[string]any{"habits": {}, "dailyLogs": {}, "workouts": {}, "settings": {}, "periods": {}}}
	if len(rows) > limit {
		out.HasMore = true
		rows = rows[:limit]
	}
	for i := range rows {
		out.Cursor = utils.EncodeCursor(rows[i].Seq) // advances over filtered rows too
		// Period data is private to the owner: never returned to viewers (and never when the actor has no owner role).
		if rows[i].Type == entitytype.Period && !a.IsOwner() {
			continue
		}
		k := changeKeys[rows[i].Type]
		out.Changes[k] = append(out.Changes[k], rows[i].Public())
	}
	return out, nil
}

// habitFromEntity converts a habit entity into scoring input.
func habitFromEntity(e *dao.Entity) scoring.Habit {
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
