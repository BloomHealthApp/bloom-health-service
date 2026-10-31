package request

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// PushOp is one operation of POST /sync/push.
type PushOp struct {
	OperationID     string         `json:"operationId"`
	EntityType      string         `json:"entityType"`
	EntityID        string         `json:"entityId"`
	Op              string         `json:"op"`
	BaseVersion     int            `json:"baseVersion"`
	Payload         map[string]any `json:"payload"`
	ClientTimestamp *time.Time     `json:"clientTimestamp"`
}

// SyncPushReq is POST /sync/push.
type SyncPushReq struct {
	DeviceID   string   `json:"deviceId"`
	Operations []PushOp `json:"operations"`
}

// Validate enforces 1-100 operations.
func (r *SyncPushReq) Validate() apperrors.IHttpError {
	if len(r.Operations) == 0 || len(r.Operations) > constants.MaxSyncOperations {
		return apperrors.BadRequestError("operations must contain 1-100 items")
	}
	return nil
}

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
