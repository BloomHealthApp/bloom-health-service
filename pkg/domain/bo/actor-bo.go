// Package bo holds the business objects passed from handlers to services and between services.
package bo

import (
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
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
