// Package model holds the domain types shared across the service.
package model

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleViewer Role = "viewer"
)

// Entity types that participate in sync.
const (
	TypeHabit    = "habit"
	TypeDailyLog = "dailyLog"
	TypeWorkout  = "workout"
	TypeSettings = "settings"
)

var EntityTypes = []string{TypeHabit, TypeDailyLog, TypeWorkout, TypeSettings}

const SettingsID = "settings"

type User struct {
	ID          string
	Email       string
	DisplayName string
	Avatar      *string
	Timezone    string
	Theme       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Space struct {
	ID        string
	Name      string
	CreatedBy string
	CreatedAt time.Time
}

type Membership struct {
	ID        string
	SpaceID   string
	UserID    string
	Role      Role
	CreatedAt time.Time
}

// Entity is a versioned, soft-deletable record scoped to a shared space.
// Type-specific fields live in Data and are validated by package entities.
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

type OTPChallenge struct {
	Email     string
	CodeHash  string
	ExpiresAt time.Time
	Attempts  int
}

type RefreshSession struct {
	ID        string
	FamilyID  string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
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
