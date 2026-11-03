package dao

import (
	"time"
)

// RefreshSession is one rotating refresh token (stored hashed) belonging to a token family.
type RefreshSession struct {
	ID        string     `bson:"_id"`
	FamilyID  string     `bson:"familyId"`
	UserID    string     `bson:"userId"`
	TokenHash string     `bson:"tokenHash"`
	ExpiresAt time.Time  `bson:"expiresAt"`
	UsedAt    *time.Time `bson:"usedAt"`
	RevokedAt *time.Time `bson:"revokedAt"`
	CreatedAt time.Time  `bson:"createdAt"`
}
