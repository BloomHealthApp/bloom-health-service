// Package bo holds the business objects passed from handlers to services and between services.
package bo

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
)

// Actor is the authenticated user with membership resolved from the database on every request.
type Actor struct {
	User       *dao.User
	Membership *dao.Membership
	// DeleteAfter is set when the actor's account, or for a viewer the space owner's, is scheduled for deletion.
	// Such an actor may only use the routes that let them see and restore the account.
	DeleteAfter *time.Time
}

// PendingDeletion reports whether the actor's access is locked by a scheduled deletion.
func (a Actor) PendingDeletion() bool { return a.DeleteAfter != nil }

// SpaceID is the actor's shared space.
func (a Actor) SpaceID() string { return a.Membership.SpaceID }

// IsOwner reports whether the actor may write.
func (a Actor) IsOwner() bool { return a.Membership.Role.IsOwner() }
