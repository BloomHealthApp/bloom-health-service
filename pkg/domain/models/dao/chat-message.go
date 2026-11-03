package dao

import (
	"time"
)

// ChatUsage is the token usage the worker reports for an assistant message.
type ChatUsage struct {
	InputTokens  int `bson:"inputTokens"`
	OutputTokens int `bson:"outputTokens"`
}

// ChatMessage is one turn of a chat session. Assistant messages double as the worker's job queue:
// the service inserts them pending and bloom-health-task-worker claims and settles them.
type ChatMessage struct {
	ID           string     `bson:"_id"`
	SessionID    string     `bson:"sessionId"`
	UserID       string     `bson:"userId"`
	Idx          int        `bson:"idx"`
	Role         string     `bson:"role"`
	Content      string     `bson:"content"`
	Status       string     `bson:"status"`
	ModelID      string     `bson:"modelId"`
	Attempt      int        `bson:"attempt"`
	RunAt        time.Time  `bson:"runAt"`
	LockedUntil  *time.Time `bson:"lockedUntil"`
	ErrorCode    string     `bson:"errorCode,omitempty"`
	Usage        *ChatUsage `bson:"usage,omitempty"`
	FinishReason string     `bson:"finishReason,omitempty"`
	LatencyMs    int        `bson:"latencyMs,omitempty"`
	CreatedAt    time.Time  `bson:"createdAt"`
	UpdatedAt    time.Time  `bson:"updatedAt"`
}
