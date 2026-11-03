package dao

import (
	"time"
)

// OpRecord stores the outcome of an applied sync operation for idempotency.
type OpRecord struct {
	OperationID string
	SpaceID     string
	Status      string
	Reason      string
	Resolved    string
	Entity      *Entity
	CreatedAt   time.Time
}
