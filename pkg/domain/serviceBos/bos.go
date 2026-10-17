// Package serviceBos holds the intermediate business objects passed between handlers and services.
package serviceBos

import (
	"time"

	"github.com/bubu-bloom/service/pkg/domain/models/dao"
)

// Actor is the authenticated user with membership resolved from the database on every request.
type Actor struct {
	User       *dao.User
	Membership *dao.Membership
}

// SpaceID is the actor's shared space.
func (a Actor) SpaceID() string { return a.Membership.SpaceID }

// IsOwner reports whether the actor may write.
func (a Actor) IsOwner() bool { return a.Membership.Role.IsOwner() }

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

// Person is a user to provision.
type Person struct{ Email, Name string }

// ProvisionOptions describes an owner/viewer pair and their shared space.
type ProvisionOptions struct {
	Owner, Viewer Person
	Timezone      string
	SpaceName     string
}
