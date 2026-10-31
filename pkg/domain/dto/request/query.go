package request

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// RangeReq is the from/to query used by logs, workouts and stats.
type RangeReq struct {
	From, To string
	// Required makes both bounds mandatory (logs).
	Required bool
}

// Validate checks the dates and the maximum range.
func (r *RangeReq) Validate() apperrors.IHttpError {
	if r.Required && (r.From == "" || r.To == "") {
		return apperrors.BadRequestError("from and to (YYYY-MM-DD) are required")
	}
	var tf, tt time.Time
	var err error
	if r.From != "" {
		if tf, err = utils.ParseDate(r.From); err != nil {
			return apperrors.BadRequestError("from must be YYYY-MM-DD")
		}
	}
	if r.To != "" {
		if tt, err = utils.ParseDate(r.To); err != nil {
			return apperrors.BadRequestError("to must be YYYY-MM-DD")
		}
	}
	if r.From != "" && r.To != "" {
		if tt.Before(tf) || tt.Sub(tf) > constants.MaxRangeDays*24*time.Hour {
			return apperrors.BadRequestError("range must be ascending and at most 366 days")
		}
	}
	return nil
}

// ListHabitsReq is GET /habits.
type ListHabitsReq struct{ IncludeInactive bool }

// Validate is a no-op.
func (r *ListHabitsReq) Validate() apperrors.IHttpError { return nil }
