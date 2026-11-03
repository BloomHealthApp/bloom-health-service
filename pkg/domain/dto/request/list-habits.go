package request

import (
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// ListHabitsReq is GET /habits.
type ListHabitsReq struct{ IncludeInactive bool }

func (r *ListHabitsReq) Validate() apperrors.IHttpError { return nil }
