// Package mongorepo defines the repository interfaces (one per aggregate) and their MongoDB
// implementations. In-memory fakes with identical semantics live in the memory subpackage.
package mongorepo

import (
	"context"

	"github.com/bubu-bloom/service/pkg/domain/models/dao"
)

// EntityQuery filters entities by an optional inclusive localDate range stored in Data["localDate"].
type EntityQuery struct {
	From, To       string
	IncludeDeleted bool
}

// IUserRepo persists users. Lookups return errors.ErrRepoNotFound when absent.
type IUserRepo interface {
	ByEmail(ctx context.Context, email string) (*dao.User, error)
	// ByPhone looks up an E.164 number (legacy users have none).
	ByPhone(ctx context.Context, phone string) (*dao.User, error)
	ByID(ctx context.Context, id string) (*dao.User, error)
	// Save upserts; returns errors.ErrRepoConflict when the email or phone belongs to another user.
	Save(ctx context.Context, u *dao.User) error
}

// ISpaceRepo persists shared spaces.
type ISpaceRepo interface {
	ByID(ctx context.Context, id string) (*dao.Space, error)
	Save(ctx context.Context, s *dao.Space) error
}

// IMembershipRepo persists memberships (unique per space+user).
type IMembershipRepo interface {
	ForUser(ctx context.Context, userID string) (*dao.Membership, error)
	MembersOfSpace(ctx context.Context, spaceID string) ([]dao.Membership, error)
	// Save returns errors.ErrRepoConflict if the user is already a member of the space.
	Save(ctx context.Context, m *dao.Membership) error
}

// IEntityRepo persists versioned entities, one collection per entity type.
type IEntityRepo interface {
	Get(ctx context.Context, spaceID, typ, id string) (*dao.Entity, error)
	List(ctx context.Context, spaceID, typ string, q EntityQuery) ([]dao.Entity, error)
	// Save inserts (e.Version == 1) or replaces the entity only if the stored version equals
	// e.Version-1 (compare-and-swap). It assigns e.Seq. Returns errors.ErrRepoConflict otherwise.
	Save(ctx context.Context, e *dao.Entity) error
	// Changes returns entities with Seq > after, ascending by Seq, including tombstones.
	Changes(ctx context.Context, spaceID string, after int64, limit int) ([]dao.Entity, error)
}

// IOpRepo stores applied sync operations for idempotency.
type IOpRepo interface {
	Get(ctx context.Context, opID string) (*dao.OpRecord, error)
	// Save returns errors.ErrRepoConflict if the operationId already exists.
	Save(ctx context.Context, r *dao.OpRecord) error
}

// IOTPRepo stores pending one-time codes keyed by challengeId.
type IOTPRepo interface {
	Save(ctx context.Context, c *dao.OTPChallenge) error
	Get(ctx context.Context, challengeID string) (*dao.OTPChallenge, error)
	// LatestByEmail returns the newest challenge mailed to email (used for the resend cooldown).
	LatestByEmail(ctx context.Context, email string) (*dao.OTPChallenge, error)
	// AddAttempt atomically increments and returns the attempt counter.
	AddAttempt(ctx context.Context, challengeID string) (int, error)
	Delete(ctx context.Context, challengeID string) error
}

// ISessionRepo stores rotating refresh sessions.
type ISessionRepo interface {
	Save(ctx context.Context, s *dao.RefreshSession) error
	ByHash(ctx context.Context, hash string) (*dao.RefreshSession, error)
	// MarkUsed atomically sets UsedAt; returns false if it was already used or revoked.
	MarkUsed(ctx context.Context, id string) (bool, error)
	RevokeFamily(ctx context.Context, familyID string) error
}
