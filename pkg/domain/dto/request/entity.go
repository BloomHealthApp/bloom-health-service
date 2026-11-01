package request

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// CreateEntityReq is POST /habits and POST /workouts. The payload is validated field by field by the
// service against a strict allowlist (mass-assignment protection).
type CreateEntityReq struct {
	Payload map[string]any
	// ID is the client-supplied id or a generated one; set by Validate.
	ID string
}

// Validate resolves and checks the entity id.
func (r *CreateEntityReq) Validate() apperrors.IHttpError {
	id, _ := r.Payload["id"].(string)
	if id == "" {
		r.ID = uuid.NewString()
		return nil
	}
	if len(id) > 128 || strings.ContainsAny(id, "/\\ :") {
		return apperrors.BadRequestError("invalid id")
	}
	r.ID = id
	return nil
}

// PatchEntityReq is PATCH /habits/{id}, PATCH /workouts/{id} and PATCH /settings.
type PatchEntityReq struct {
	ID      string
	Payload map[string]any
	// Version is the optimistic-concurrency version; HasVersion is false when it was missing or invalid.
	Version    int
	HasVersion bool
}

// Validate extracts the version from the payload (it is not part of the entity data).
func (r *PatchEntityReq) Validate() apperrors.IHttpError {
	if r.Payload == nil {
		r.Payload = map[string]any{}
	}
	if f, ok := r.Payload["version"].(float64); ok && f >= 1 && f == float64(int(f)) {
		r.Version, r.HasVersion = int(f), true
	}
	delete(r.Payload, "version")
	delete(r.Payload, "id")
	return nil
}

// PutLogReq is PUT /logs/{habitId}/{localDate}.
type PutLogReq struct {
	HabitID     string   `json:"-"`
	LocalDate   string   `json:"-"`
	Value       *float64 `json:"value"`
	Status      string   `json:"status"`
	Note        *string  `json:"note"`
	Source      string   `json:"source"`
	BaseVersion int      `json:"baseVersion"`
}

// Validate is a no-op: date, habit and value are validated by the entity rules.
func (r *PutLogReq) Validate() apperrors.IHttpError { return nil }
