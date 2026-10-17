// Package store defines persistence behind an interface with in-memory and MongoDB implementations.
package store

import (
	"context"
	"errors"

	"github.com/bubu-bloom/service/internal/model"
)

var (
	ErrNotFound = errors.New("not found")
	// ErrConflict signals an optimistic-concurrency or uniqueness failure.
	ErrConflict = errors.New("version conflict")
)

// ListQuery filters entities by an optional inclusive localDate range stored in Data["localDate"].
type ListQuery struct {
	From, To       string
	IncludeDeleted bool
}

type Store interface {
	Ping(ctx context.Context) error

	UserByEmail(ctx context.Context, email string) (*model.User, error)
	UserByID(ctx context.Context, id string) (*model.User, error)
	SaveUser(ctx context.Context, u *model.User) error

	SpaceByID(ctx context.Context, id string) (*model.Space, error)
	SaveSpace(ctx context.Context, s *model.Space) error
	MembershipForUser(ctx context.Context, userID string) (*model.Membership, error)
	MembersOfSpace(ctx context.Context, spaceID string) ([]model.Membership, error)
	SaveMembership(ctx context.Context, m *model.Membership) error

	GetEntity(ctx context.Context, spaceID, typ, id string) (*model.Entity, error)
	ListEntities(ctx context.Context, spaceID, typ string, q ListQuery) ([]model.Entity, error)
	// SaveEntity inserts (e.Version == 1) or replaces the entity if the stored version equals e.Version-1.
	// It assigns e.Seq. Returns ErrConflict otherwise.
	SaveEntity(ctx context.Context, e *model.Entity) error
	// Changes returns entities with Seq > after, ascending, including tombstones.
	Changes(ctx context.Context, spaceID string, after int64, limit int) ([]model.Entity, error)

	GetOp(ctx context.Context, opID string) (*model.OpRecord, error)
	// SaveOp stores the record; returns ErrConflict if the operationId already exists.
	SaveOp(ctx context.Context, r *model.OpRecord) error

	SaveOTP(ctx context.Context, c *model.OTPChallenge) error
	GetOTP(ctx context.Context, email string) (*model.OTPChallenge, error)
	AddOTPAttempt(ctx context.Context, email string) (int, error)
	DeleteOTP(ctx context.Context, email string) error

	SaveSession(ctx context.Context, s *model.RefreshSession) error
	SessionByHash(ctx context.Context, hash string) (*model.RefreshSession, error)
	// MarkSessionUsed atomically sets UsedAt; returns false if it was already used or revoked.
	MarkSessionUsed(ctx context.Context, id string) (bool, error)
	RevokeFamily(ctx context.Context, familyID string) error
}
