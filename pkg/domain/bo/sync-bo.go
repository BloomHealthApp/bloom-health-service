package bo

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
)

// ApplyOp is one write operation. OperationID may be empty for direct REST writes.
type ApplyOp struct {
	OperationID     string
	EntityType      string
	EntityID        string
	Op              string // upsert | delete
	BaseVersion     int
	Payload         map[string]any
	ClientTimestamp *time.Time
}

// ApplyResult is the outcome of an ApplyOp. Status is an opstatus value.
type ApplyResult struct {
	OperationID string
	Status      string
	Reason      string
	Resolved    string
	Entity      *dao.Entity
}
