package dao

import (
	"time"
)

// Space is the shared space owner and viewer collaborate in.
type Space struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	CreatedBy string    `bson:"createdBy"`
	CreatedAt time.Time `bson:"createdAt"`
}
