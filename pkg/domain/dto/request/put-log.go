package request

import (
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

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
