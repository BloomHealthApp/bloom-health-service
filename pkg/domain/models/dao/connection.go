package dao

import (
	"sort"
	"strings"
	"time"
)

// Connection statuses. A declined request is kept only as a cooldown marker until ExpiresAt.
const (
	ConnectionPending  = "pending"
	ConnectionAccepted = "accepted"
	ConnectionDeclined = "declined"
)

// Connection is a friend request between two users, or (once accepted) a mutual friendship. There is at most one per
// pair of users, in either direction (PairKey is unique).
type Connection struct {
	ID          string     `bson:"_id"`
	PairKey     string     `bson:"pairKey"`
	RequesterID string     `bson:"requesterId"`
	AddresseeID string     `bson:"addresseeId"`
	Status      string     `bson:"status"`
	CreatedAt   time.Time  `bson:"createdAt"`
	RespondedAt *time.Time `bson:"respondedAt,omitempty"`
	// ExpiresAt is set only on a declined request; the TTL index removes the marker then, ending the cooldown.
	ExpiresAt *time.Time `bson:"expiresAt,omitempty"`
}

// PairKey is the same for (a, b) and (b, a).
func PairKey(a, b string) string {
	ids := []string{a, b}
	sort.Strings(ids)
	return strings.Join(ids, "|")
}

// Other returns the user on the other side of the connection from userID.
func (c *Connection) Other(userID string) string {
	if c.RequesterID == userID {
		return c.AddresseeID
	}
	return c.RequesterID
}

// Involves reports whether userID is one of the two users.
func (c *Connection) Involves(userID string) bool {
	return c.RequesterID == userID || c.AddresseeID == userID
}
