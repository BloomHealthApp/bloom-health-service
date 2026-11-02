// Package dao holds the persisted documents. Simple aggregates map directly to BSON; entities and
// operation records are converted by the Mongo repositories because they embed free-form data.
package dao

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
)

// User is an account, created on first verified login.
type User struct {
	ID         string `bson:"_id"`
	Email      string `bson:"email"`
	EmailLower string `bson:"emailLower"`
	// Phone is E.164 and unique; nil (field absent) for legacy accounts created before phone sign-up.
	Phone       *string `bson:"phone,omitempty"`
	DisplayName string  `bson:"displayName"`
	Avatar      *string `bson:"avatar"`
	// Gender is one of pkg/enums/gender; nil (null) for legacy accounts or when not provided.
	Gender *string `bson:"gender,omitempty"`
	// PeriodTracker is the explicit period-tracker setting; nil means "default for the gender"
	// (on for female). Use IsPeriodTrackerEnabled for the effective value.
	PeriodTracker *bool     `bson:"periodTracker,omitempty"`
	Timezone      string    `bson:"timezone"`
	Theme         string    `bson:"theme"`
	CreatedAt     time.Time `bson:"createdAt"`
	UpdatedAt     time.Time `bson:"updatedAt"`
}

// IsPeriodTrackerEnabled is the effective tracker setting: the explicit choice if set, else gender == female.
func (u *User) IsPeriodTrackerEnabled() bool {
	if u == nil {
		return false
	}
	if u.PeriodTracker != nil {
		return *u.PeriodTracker
	}
	return u.Gender != nil && *u.Gender == "female"
}

// Space is the shared space owner and viewer collaborate in.
type Space struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	CreatedBy string    `bson:"createdBy"`
	CreatedAt time.Time `bson:"createdAt"`
}

// Membership links a user to a space with a role. The role is the source of truth for authorization.
type Membership struct {
	ID        string        `bson:"_id"`
	SpaceID   string        `bson:"sharedSpaceId"`
	UserID    string        `bson:"userId"`
	Role      userrole.Role `bson:"role"`
	CreatedAt time.Time     `bson:"createdAt"`
}

// Entity is a versioned, soft-deletable record scoped to a shared space. Type-specific fields live in
// Data and are validated by infrastructure/validation.
type Entity struct {
	ID        string
	Type      string
	SpaceID   string
	Version   int
	Seq       int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	Data      map[string]any
}

// Public renders the entity as the API representation.
func (e *Entity) Public() map[string]any {
	out := make(map[string]any, len(e.Data)+6)
	for k, v := range e.Data {
		out[k] = v
	}
	out["id"] = e.ID
	out["version"] = e.Version
	out["createdAt"] = e.CreatedAt.UTC().Format(time.RFC3339)
	out["updatedAt"] = e.UpdatedAt.UTC().Format(time.RFC3339)
	if e.DeletedAt != nil {
		out["deletedAt"] = e.DeletedAt.UTC().Format(time.RFC3339)
	} else {
		out["deletedAt"] = nil
	}
	return out
}

// Clone deep-copies the top level of the entity.
func (e *Entity) Clone() *Entity {
	c := *e
	c.Data = make(map[string]any, len(e.Data))
	for k, v := range e.Data {
		c.Data[k] = v
	}
	if e.DeletedAt != nil {
		t := *e.DeletedAt
		c.DeletedAt = &t
	}
	return &c
}

// OTP challenge purposes.
const (
	OTPPurposeSignUp = "signup"
	OTPPurposeSignIn = "signin"
	// OTPPurposePhoneChange proves a signed-in user controls the account email before a phone is saved.
	OTPPurposePhoneChange = "phone_change"
)

// OTPChallenge is a pending one-time code (stored hashed), addressed by its opaque ChallengeID.
// Sign-up challenges carry the pending name and phone; the account exists only after verification.
type OTPChallenge struct {
	ChallengeID   string    `bson:"_id"`
	Purpose       string    `bson:"purpose"`
	Email         string    `bson:"email"` // lower-cased address the code was mailed to
	PendingName   string    `bson:"pendingName,omitempty"`
	PendingPhone  string    `bson:"pendingPhone,omitempty"`
	PendingAvatar string    `bson:"pendingAvatar,omitempty"` // optional avatar id chosen at sign-up
	PendingGender string    `bson:"pendingGender,omitempty"` // optional gender chosen at sign-up
	CodeHash      string    `bson:"codeHash"`
	CreatedAt     time.Time `bson:"createdAt"`
	ExpiresAt     time.Time `bson:"expiresAt"`
	Attempts      int       `bson:"attempts"`
}

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
