package request

import (
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// SyncPullReq is GET /sync/pull.
type SyncPullReq struct {
	Cursor   string
	RawLimit string
	// Limit and After are derived by Validate.
	Limit int
	After int64
}

// Validate parses the limit (digits only) and the opaque cursor.
func (r *SyncPullReq) Validate() apperrors.IHttpError {
	if r.RawLimit != "" {
		n := 0
		for _, c := range r.RawLimit {
			if c < '0' || c > '9' || n > 100000 {
				return apperrors.BadRequestError("invalid limit")
			}
			n = n*10 + int(c-'0')
		}
		r.Limit = n
	}
	after, err := utils.DecodeCursor(r.Cursor)
	if err != nil {
		return apperrors.BadRequestError("invalid cursor")
	}
	r.After = after
	return nil
}
