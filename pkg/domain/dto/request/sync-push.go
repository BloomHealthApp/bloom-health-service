package request

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
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
