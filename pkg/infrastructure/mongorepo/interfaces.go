// Package mongorepo defines the repository interfaces (one per aggregate) and their MongoDB
// implementations. In-memory fakes with identical semantics live in the memory subpackage.
package mongorepo

import (
	"context"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
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

// IChatSessionRepo persists chat sessions. Every lookup is scoped to the owning user, and
// soft-deleted sessions are reported as errors.ErrRepoNotFound.
type IChatSessionRepo interface {
	Create(ctx context.Context, s *dao.ChatSession) error
	Get(ctx context.Context, userID, id string) (*dao.ChatSession, error)
	// ListForUser returns non-deleted sessions, most recently updated first.
	ListForUser(ctx context.Context, userID string, limit int) ([]dao.ChatSession, error)
	// SoftDelete marks the session deleted and clears its title. The tombstone expires after constants.DeletedChatRetention.
	SoftDelete(ctx context.Context, userID, id string, at time.Time) error
	// BeginTurn marks turnID as the session's in-flight turn and reserves two message indexes,
	// returning the first. errors.ErrRepoConflict when another turn is already in flight.
	BeginTurn(ctx context.Context, userID, id, turnID string, at time.Time) (int, error)
	// EndTurn clears the in-flight turn only if it is still turnID.
	EndTurn(ctx context.Context, id, turnID string) error
	SetTitle(ctx context.Context, userID, id, title string) error
}

// IChatMessageRepo persists chat messages.
type IChatMessageRepo interface {
	Insert(ctx context.Context, msgs ...*dao.ChatMessage) error
	// ListForSession returns up to limit messages with Idx < beforeIdx (all when beforeIdx <= 0),
	// newest first.
	ListForSession(ctx context.Context, sessionID string, beforeIdx, limit int) ([]dao.ChatMessage, error)
	// CountUserMessagesSince counts the user-role messages userID sent at or after since.
	CountUserMessagesSince(ctx context.Context, userID string, since time.Time) (int, error)
	// DeleteForSession hard-deletes every message of a session.
	DeleteForSession(ctx context.Context, sessionID string) error
}

// IAccountRepo hard-deletes accounts (DELETE /v1/me). Every method is idempotent, so a failed erase can be retried.
type IAccountRepo interface {
	// EraseSpace deletes a space and everything stored in it: entities of every type, applied operations, its sync
	// counter and all its memberships.
	EraseSpace(ctx context.Context, spaceID string) error
	// EraseUser deletes the user and everything tied to them: memberships, refresh sessions, pending codes mailed
	// to their email, and their chat sessions and messages.
	EraseUser(ctx context.Context, user *dao.User) error
}
