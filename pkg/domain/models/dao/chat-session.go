package dao

import (
	"time"
)

// ChatSession is one AI chat conversation, owned by a single user. Its shape is shared with
// bloom-health-task-worker (docs/chat-contract.md), so field names must not change on one side only.
type ChatSession struct {
	ID      string `bson:"_id"`
	UserID  string `bson:"userId"`
	SpaceID string `bson:"spaceId"`
	Title   string `bson:"title"`
	ModelID string `bson:"modelId"`
	// ContextScopes names the user data the worker may add to the prompt; always empty in v1.
	ContextScopes []string `bson:"contextScopes"`
	// ActiveTurnID is the id of the pending assistant message; nil when no turn is in flight.
	ActiveTurnID *string    `bson:"activeTurnId"`
	NextIdx      int        `bson:"nextIdx"`
	Version      int        `bson:"version"`
	CreatedAt    time.Time  `bson:"createdAt"`
	UpdatedAt    time.Time  `bson:"updatedAt"`
	DeletedAt    *time.Time `bson:"deletedAt"`
}
