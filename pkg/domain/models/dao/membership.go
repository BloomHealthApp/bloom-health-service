package dao

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
)

// Membership links a user to a space with a role. The role is the source of truth for authorization.
type Membership struct {
	ID        string        `bson:"_id"`
	SpaceID   string        `bson:"sharedSpaceId"`
	UserID    string        `bson:"userId"`
	Role      userrole.Role `bson:"role"`
	CreatedAt time.Time     `bson:"createdAt"`
}
