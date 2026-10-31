// Package memory provides in-memory fakes of the mongorepo repositories. They follow the same
// semantics (typed errors, CAS-on-version, per-space seq, unique constraints) and back tests and
// the STORE=memory development mode.
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

// Pinger is an always-healthy drivers.IPinger.
type Pinger struct{}

// Ping always succeeds.
func (Pinger) Ping(context.Context) error { return nil }

// ---- users ----

type userRepo struct {
	mu    sync.Mutex
	users map[string]*dao.User
}

// NewUserRepo returns an in-memory IUserRepo.
func NewUserRepo() mongorepo.IUserRepo { return &userRepo{users: map[string]*dao.User{}} }

func (r *userRepo) ByEmail(_ context.Context, email string) (*dao.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			c := *u
			return &c, nil
		}
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *userRepo) ByPhone(_ context.Context, phone string) (*dao.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.Phone != nil && *u.Phone == phone {
			c := *u
			return &c, nil
		}
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *userRepo) ByID(_ context.Context, id string) (*dao.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[id]; ok {
		c := *u
		return &c, nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *userRepo) Save(_ context.Context, u *dao.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.users {
		if x.ID == u.ID {
			continue
		}
		if strings.EqualFold(x.Email, u.Email) || (u.Phone != nil && x.Phone != nil && *x.Phone == *u.Phone) {
			return apperrors.ErrRepoConflict
		}
	}
	c := *u
	c.EmailLower = strings.ToLower(u.Email)
	r.users[u.ID] = &c
	return nil
}

// ---- spaces ----

type spaceRepo struct {
	mu     sync.Mutex
	spaces map[string]*dao.Space
}

// NewSpaceRepo returns an in-memory ISpaceRepo.
func NewSpaceRepo() mongorepo.ISpaceRepo { return &spaceRepo{spaces: map[string]*dao.Space{}} }

func (r *spaceRepo) ByID(_ context.Context, id string) (*dao.Space, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.spaces[id]; ok {
		c := *s
		return &c, nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *spaceRepo) Save(_ context.Context, s *dao.Space) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *s
	r.spaces[s.ID] = &c
	return nil
}

// ---- memberships ----

type membershipRepo struct {
	mu sync.Mutex
	m  map[string]*dao.Membership
}

// NewMembershipRepo returns an in-memory IMembershipRepo.
func NewMembershipRepo() mongorepo.IMembershipRepo {
	return &membershipRepo{m: map[string]*dao.Membership{}}
}

func (r *membershipRepo) ForUser(_ context.Context, userID string) (*dao.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, mm := range r.m {
		if mm.UserID == userID {
			c := *mm
			return &c, nil
		}
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *membershipRepo) MembersOfSpace(_ context.Context, spaceID string) ([]dao.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []dao.Membership
	for _, mm := range r.m {
		if mm.SpaceID == spaceID {
			out = append(out, *mm)
		}
	}
	return out, nil
}

func (r *membershipRepo) Save(_ context.Context, mm *dao.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.m {
		if x.SpaceID == mm.SpaceID && x.UserID == mm.UserID && x.ID != mm.ID {
			return apperrors.ErrRepoConflict
		}
	}
	c := *mm
	r.m[mm.ID] = &c
	return nil
}

// ---- entities ----

type entityRepo struct {
	mu       sync.Mutex
	entities map[string]*dao.Entity
	seq      map[string]int64
}

// NewEntityRepo returns an in-memory IEntityRepo.
func NewEntityRepo() mongorepo.IEntityRepo {
	return &entityRepo{entities: map[string]*dao.Entity{}, seq: map[string]int64{}}
}

func ekey(space, typ, id string) string { return space + "|" + typ + "|" + id }

func (r *entityRepo) Get(_ context.Context, space, typ, id string) (*dao.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entities[ekey(space, typ, id)]; ok {
		return e.Clone(), nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *entityRepo) List(_ context.Context, space, typ string, q mongorepo.EntityQuery) ([]dao.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []dao.Entity
	for _, e := range r.entities {
		if e.SpaceID != space || e.Type != typ {
			continue
		}
		if e.DeletedAt != nil && !q.IncludeDeleted {
			continue
		}
		if q.From != "" || q.To != "" {
			d, _ := e.Data["localDate"].(string)
			if d == "" || (q.From != "" && d < q.From) || (q.To != "" && d > q.To) {
				continue
			}
		}
		out = append(out, *e.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt) || (out[i].CreatedAt.Equal(out[j].CreatedAt) && out[i].ID < out[j].ID)
	})
	return out, nil
}

func (r *entityRepo) Save(_ context.Context, e *dao.Entity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := ekey(e.SpaceID, e.Type, e.ID)
	cur, exists := r.entities[k]
	switch {
	case e.Version == 1 && exists:
		return apperrors.ErrRepoConflict
	case e.Version > 1 && (!exists || cur.Version != e.Version-1):
		return apperrors.ErrRepoConflict
	}
	r.seq[e.SpaceID]++
	e.Seq = r.seq[e.SpaceID]
	r.entities[k] = e.Clone()
	return nil
}

func (r *entityRepo) Changes(_ context.Context, space string, after int64, limit int) ([]dao.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []dao.Entity
	for _, e := range r.entities {
		if e.SpaceID == space && e.Seq > after {
			out = append(out, *e.Clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---- operations ----

type opRepo struct {
	mu  sync.Mutex
	ops map[string]*dao.OpRecord
}

// NewOpRepo returns an in-memory IOpRepo.
func NewOpRepo() mongorepo.IOpRepo { return &opRepo{ops: map[string]*dao.OpRecord{}} }

func (r *opRepo) Get(_ context.Context, id string) (*dao.OpRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.ops[id]; ok {
		return cloneOp(rec), nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *opRepo) Save(_ context.Context, rec *dao.OpRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.ops[rec.OperationID]; ok {
		return apperrors.ErrRepoConflict
	}
	r.ops[rec.OperationID] = cloneOp(rec)
	return nil
}

func cloneOp(rec *dao.OpRecord) *dao.OpRecord {
	c := *rec
	if rec.Entity != nil {
		c.Entity = rec.Entity.Clone()
	}
	return &c
}

// ---- OTP ----

type otpRepo struct {
	mu   sync.Mutex
	otps map[string]*dao.OTPChallenge
}

// NewOTPRepo returns an in-memory IOTPRepo.
func NewOTPRepo() mongorepo.IOTPRepo { return &otpRepo{otps: map[string]*dao.OTPChallenge{}} }

func (r *otpRepo) Save(_ context.Context, c *dao.OTPChallenge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x := *c
	x.Email = strings.ToLower(c.Email)
	r.otps[c.ChallengeID] = &x
	return nil
}

func (r *otpRepo) Get(_ context.Context, challengeID string) (*dao.OTPChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.otps[challengeID]; ok {
		x := *c
		return &x, nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *otpRepo) LatestByEmail(_ context.Context, email string) (*dao.OTPChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var best *dao.OTPChallenge
	for _, c := range r.otps {
		if c.Email == strings.ToLower(email) && (best == nil || c.CreatedAt.After(best.CreatedAt)) {
			best = c
		}
	}
	if best == nil {
		return nil, apperrors.ErrRepoNotFound
	}
	x := *best
	return &x, nil
}

func (r *otpRepo) AddAttempt(_ context.Context, challengeID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.otps[challengeID]
	if !ok {
		return 0, apperrors.ErrRepoNotFound
	}
	c.Attempts++
	return c.Attempts, nil
}

func (r *otpRepo) Delete(_ context.Context, challengeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.otps, challengeID)
	return nil
}

// ---- sessions ----

type sessionRepo struct {
	mu       sync.Mutex
	sessions map[string]*dao.RefreshSession
}

// NewSessionRepo returns an in-memory ISessionRepo.
func NewSessionRepo() mongorepo.ISessionRepo {
	return &sessionRepo{sessions: map[string]*dao.RefreshSession{}}
}

func (r *sessionRepo) Save(_ context.Context, s *dao.RefreshSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.sessions {
		if x.TokenHash == s.TokenHash {
			return apperrors.ErrRepoConflict
		}
	}
	c := *s
	r.sessions[s.ID] = &c
	return nil
}

func (r *sessionRepo) ByHash(_ context.Context, hash string) (*dao.RefreshSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if s.TokenHash == hash {
			c := *s
			return &c, nil
		}
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *sessionRepo) MarkUsed(_ context.Context, id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok || s.UsedAt != nil || s.RevokedAt != nil {
		return false, nil
	}
	now := time.Now().UTC()
	s.UsedAt = &now
	return true, nil
}

func (r *sessionRepo) RevokeFamily(_ context.Context, family string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, s := range r.sessions {
		if s.FamilyID == family && s.RevokedAt == nil {
			t := now
			s.RevokedAt = &t
		}
	}
	return nil
}
