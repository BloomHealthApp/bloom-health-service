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
