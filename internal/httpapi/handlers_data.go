package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bubu-bloom/service/internal/entities"
	"github.com/bubu-bloom/service/internal/model"
	"github.com/bubu-bloom/service/internal/scoring"
	"github.com/bubu-bloom/service/internal/service"
	"github.com/bubu-bloom/service/internal/store"
)

// respondResult maps a service result onto an HTTP response for direct REST writes.
func (s *Server) respondResult(w http.ResponseWriter, r *http.Request, res service.Result, okStatus int) {
	switch res.Status {
	case service.StatusApplied, service.StatusDuplicate:
		writeJSON(w, okStatus, res.Entity.Public())
	case service.StatusConflict:
		writeErr(w, http.StatusConflict, "conflict", "the record was changed elsewhere", map[string]any{"server": res.Entity.Public()})
	case service.StatusRejected:
		switch {
		case res.Reason == "forbidden":
			writeErr(w, 403, "forbidden", "read-only access", nil)
		case res.Reason == "not_found":
			writeErr(w, 404, "not_found", "not found", nil)
		default:
			writeErr(w, 400, "validation_failed", strings.TrimPrefix(res.Reason, "validation_failed: "), nil)
		}
	default:
		s.internal(w, r, res.Err)
	}
}

func queryRange(w http.ResponseWriter, r *http.Request, required bool) (from, to string, ok bool) {
	from, to = r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if required && (from == "" || to == "") {
		writeErr(w, 400, "validation_failed", "from and to (YYYY-MM-DD) are required", nil)
		return "", "", false
	}
	var tf, tt time.Time
	var err error
	if from != "" {
		if tf, err = entities.ParseDate(from); err != nil {
			writeErr(w, 400, "validation_failed", "from must be YYYY-MM-DD", nil)
			return "", "", false
		}
	}
	if to != "" {
		if tt, err = entities.ParseDate(to); err != nil {
			writeErr(w, 400, "validation_failed", "to must be YYYY-MM-DD", nil)
			return "", "", false
		}
	}
	if from != "" && to != "" {
		if tt.Before(tf) || tt.Sub(tf) > 366*24*time.Hour {
			writeErr(w, 400, "validation_failed", "range must be ascending and at most 366 days", nil)
			return "", "", false
		}
	}
	return from, to, true
}

func publicList(es []model.Entity) []map[string]any {
	out := make([]map[string]any, 0, len(es))
	for i := range es {
		out = append(out, es[i].Public())
	}
	return out
}

func versionField(body map[string]any) (int, bool) {
	f, ok := body["version"].(float64)
	if !ok || f < 1 || f != float64(int(f)) {
		return 0, false
	}
	delete(body, "version")
	return int(f), true
}

// ---- settings ----

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	a := actorFrom(r)
	e, err := s.Store.GetEntity(r.Context(), a.SpaceID(), model.TypeSettings, model.SettingsID)
	if errors.Is(err, store.ErrNotFound) {
		data, _ := entities.Merge(model.TypeSettings, nil, entities.DefaultSettings(), entities.Context{})
		now := time.Now().UTC()
		writeJSON(w, 200, (&model.Entity{ID: model.SettingsID, Data: data, CreatedAt: now, UpdatedAt: now}).Public())
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, e.Public())
}

func (s *Server) patchSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if !decode(w, r, &body) {
		return
	}
	v, ok := versionField(body)
	if !ok {
		v = 0
		delete(body, "version")
	}
	res := s.Svc.Apply(r.Context(), actorFrom(r), service.Op{EntityType: model.TypeSettings, EntityID: model.SettingsID, Op: "upsert", BaseVersion: v, Payload: body})
	s.respondResult(w, r, res, 200)
}

// ---- habits ----

func (s *Server) listHabits(w http.ResponseWriter, r *http.Request) {
	a := actorFrom(r)
	es, err := s.Store.ListEntities(r.Context(), a.SpaceID(), model.TypeHabit, store.ListQuery{})
	if err != nil {
		s.internal(w, r, err)
		return
	}
	incl := r.URL.Query().Get("includeInactive") == "true"
	out := es[:0]
	for _, e := range es {
		if active, _ := e.Data["isActive"].(bool); active || incl {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := out[i].Data["sortOrder"].(float64)
		b, _ := out[j].Data["sortOrder"].(float64)
		return a < b
	})
	writeJSON(w, 200, map[string]any{"items": publicList(out)})
}

func clientID(body map[string]any) (string, bool) {
	id, _ := body["id"].(string)
	if id == "" {
		return uuid.NewString(), true
	}
	return id, len(id) <= 128 && !strings.ContainsAny(id, "/\\ :")
}

func (s *Server) create(w http.ResponseWriter, r *http.Request, typ string) {
	var body map[string]any
	if !decode(w, r, &body) {
		return
	}
	id, ok := clientID(body)
	if !ok {
		writeErr(w, 400, "validation_failed", "invalid id", nil)
		return
	}
	res := s.Svc.Apply(r.Context(), actorFrom(r), service.Op{EntityType: typ, EntityID: id, Op: "upsert", BaseVersion: 0, Payload: body})
	s.respondResult(w, r, res, http.StatusCreated)
}

func (s *Server) patch(w http.ResponseWriter, r *http.Request, typ string) {
	var body map[string]any
	if !decode(w, r, &body) {
		return
	}
	v, ok := versionField(body)
	if !ok {
		writeErr(w, 400, "validation_failed", "version is required", nil)
		return
	}
	delete(body, "id")
	res := s.Svc.Apply(r.Context(), actorFrom(r), service.Op{EntityType: typ, EntityID: chi.URLParam(r, "id"), Op: "upsert", BaseVersion: v, Payload: body})
	s.respondResult(w, r, res, 200)
}

func (s *Server) createHabit(w http.ResponseWriter, r *http.Request) { s.create(w, r, model.TypeHabit) }
func (s *Server) patchHabit(w http.ResponseWriter, r *http.Request) {
	s.patchExisting(w, r, model.TypeHabit)
}

// patchExisting verifies the entity exists first so PATCH never creates records.
func (s *Server) patchExisting(w http.ResponseWriter, r *http.Request, typ string) {
	a := actorFrom(r)
	if _, err := s.Store.GetEntity(r.Context(), a.SpaceID(), typ, chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, 404, "not_found", "not found", nil)
			return
		}
		s.internal(w, r, err)
		return
	}
	s.patch(w, r, typ)
}

func (s *Server) deleteHabit(w http.ResponseWriter, r *http.Request) {
	a := actorFrom(r)
	id := chi.URLParam(r, "id")
	e, err := s.Store.GetEntity(r.Context(), a.SpaceID(), model.TypeHabit, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, 404, "not_found", "not found", nil)
			return
		}
		s.internal(w, r, err)
		return
	}
	res := s.Svc.Apply(r.Context(), a, service.Op{EntityType: model.TypeHabit, EntityID: id, Op: "upsert", BaseVersion: e.Version, Payload: map[string]any{"isActive": false}})
	if res.Status == service.StatusApplied {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.respondResult(w, r, res, 200)
}

// ---- logs ----

func (s *Server) listLogs(w http.ResponseWriter, r *http.Request) {
	from, to, ok := queryRange(w, r, true)
	if !ok {
		return
	}
	es, err := s.Store.ListEntities(r.Context(), actorFrom(r).SpaceID(), model.TypeDailyLog, store.ListQuery{From: from, To: to, IncludeDeleted: true})
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": publicList(es)})
}

func (s *Server) putLog(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Value       *float64 `json:"value"`
		Status      string   `json:"status"`
		Note        *string  `json:"note"`
		BaseVersion int      `json:"baseVersion"`
	}
	if !decode(w, r, &b) {
		return
	}
	payload := map[string]any{"value": nil, "note": nil}
	if b.Value != nil {
		payload["value"] = *b.Value
	}
	if b.Note != nil {
		payload["note"] = *b.Note
	}
	if b.Status != "" {
		payload["status"] = b.Status
	}
	hid, date := chi.URLParam(r, "habitId"), chi.URLParam(r, "localDate")
	res := s.Svc.Apply(r.Context(), actorFrom(r), service.Op{EntityType: model.TypeDailyLog, EntityID: service.LogID(hid, date), Op: "upsert", BaseVersion: b.BaseVersion, Payload: payload})
	s.respondResult(w, r, res, 200)
}

// ---- workouts ----

func (s *Server) listWorkouts(w http.ResponseWriter, r *http.Request) {
	from, to, ok := queryRange(w, r, false)
	if !ok {
		return
	}
	es, err := s.Store.ListEntities(r.Context(), actorFrom(r).SpaceID(), model.TypeWorkout, store.ListQuery{From: from, To: to})
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": publicList(es)})
}

func (s *Server) createWorkout(w http.ResponseWriter, r *http.Request) {
	s.create(w, r, model.TypeWorkout)
}
func (s *Server) patchWorkout(w http.ResponseWriter, r *http.Request) {
	s.patchExisting(w, r, model.TypeWorkout)
}

func (s *Server) deleteWorkout(w http.ResponseWriter, r *http.Request) {
	a := actorFrom(r)
	id := chi.URLParam(r, "id")
	e, err := s.Store.GetEntity(r.Context(), a.SpaceID(), model.TypeWorkout, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, 404, "not_found", "not found", nil)
			return
		}
		s.internal(w, r, err)
		return
	}
	res := s.Svc.Apply(r.Context(), a, service.Op{EntityType: model.TypeWorkout, EntityID: id, Op: "delete", BaseVersion: e.Version})
	if res.Status == service.StatusApplied {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.respondResult(w, r, res, 200)
}

// ---- stats ----

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	a := actorFrom(r)
	from, to, ok := queryRange(w, r, false)
	if !ok {
		return
	}
	today := scoring.LocalDate(time.Now(), s.Svc.OwnerTimezone(r.Context(), a.SpaceID()))
	if to == "" {
		to = today
	}
	if from == "" {
		t, _ := time.Parse("2006-01-02", to)
		from = t.AddDate(0, 0, -29).Format("2006-01-02")
	}
	sum, err := s.Svc.Summary(r.Context(), a.SpaceID(), from, to)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, sum)
}

// ---- sync ----

type pushOp struct {
	OperationID     string         `json:"operationId"`
	EntityType      string         `json:"entityType"`
	EntityID        string         `json:"entityId"`
	Op              string         `json:"op"`
	BaseVersion     int            `json:"baseVersion"`
	Payload         map[string]any `json:"payload"`
	ClientTimestamp *time.Time     `json:"clientTimestamp"`
}

func (s *Server) syncPush(w http.ResponseWriter, r *http.Request) {
	var b struct {
		DeviceID   string   `json:"deviceId"`
		Operations []pushOp `json:"operations"`
	}
	if !decode(w, r, &b) {
		return
	}
	if len(b.Operations) == 0 || len(b.Operations) > 100 {
		writeErr(w, 400, "validation_failed", "operations must contain 1-100 items", nil)
		return
	}
	a := actorFrom(r)
	results := make([]map[string]any, 0, len(b.Operations))
	for _, o := range b.Operations {
		if o.OperationID == "" || len(o.OperationID) > 64 {
			results = append(results, map[string]any{"operationId": o.OperationID, "status": service.StatusRejected, "reason": "invalid_operation_id"})
			continue
		}
		res := s.Svc.Apply(r.Context(), a, service.Op{OperationID: o.OperationID, EntityType: o.EntityType, EntityID: o.EntityID,
			Op: o.Op, BaseVersion: o.BaseVersion, Payload: o.Payload, ClientTimestamp: o.ClientTimestamp})
		if res.Err != nil {
			s.internal(w, r, res.Err)
			return
		}
		item := map[string]any{"operationId": o.OperationID, "status": res.Status}
		if res.Entity != nil {
			item["entity"] = res.Entity.Public()
			item["entityType"] = res.Entity.Type
		}
		if res.Reason != "" {
			item["reason"] = res.Reason
		}
		if res.Resolved != "" {
			item["resolved"] = res.Resolved
		}
		results = append(results, item)
	}
	writeJSON(w, 200, map[string]any{"results": results})
}

func (s *Server) syncPull(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		n := 0
		for _, c := range l {
			if c < '0' || c > '9' || n > 100000 {
				writeErr(w, 400, "validation_failed", "invalid limit", nil)
				return
			}
			n = n*10 + int(c-'0')
		}
		limit = n
	}
	page, err := s.Svc.Pull(r.Context(), actorFrom(r).SpaceID(), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		if strings.Contains(err.Error(), "invalid cursor") {
			writeErr(w, 400, "validation_failed", "invalid cursor", nil)
			return
		}
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"cursor": page.Cursor, "hasMore": page.HasMore, "changes": page.Changes})
}
