package store

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/bubu-bloom/service/internal/model"
)

// Memory is a thread-safe in-memory Store used by tests and local development.
type Memory struct {
	mu          sync.Mutex
	users       map[string]*model.User
	spaces      map[string]*model.Space
	memberships map[string]*model.Membership
	entities    map[string]*model.Entity
	seq         map[string]int64
	ops         map[string]*model.OpRecord
	otps        map[string]*model.OTPChallenge
	sessions    map[string]*model.RefreshSession
}

func NewMemory() *Memory {
	return &Memory{
		users: map[string]*model.User{}, spaces: map[string]*model.Space{},
		memberships: map[string]*model.Membership{}, entities: map[string]*model.Entity{},
		seq: map[string]int64{}, ops: map[string]*model.OpRecord{},
		otps: map[string]*model.OTPChallenge{}, sessions: map[string]*model.RefreshSession{},
	}
}

func ekey(space, typ, id string) string { return space + "|" + typ + "|" + id }

func (m *Memory) Ping(context.Context) error { return nil }

func (m *Memory) UserByEmail(_ context.Context, email string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if strings.EqualFold(u.Email, email) {
			c := *u
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) UserByID(_ context.Context, id string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		c := *u
		return &c, nil
	}
	return nil, ErrNotFound
}

func (m *Memory) SaveUser(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *u
	m.users[u.ID] = &c
	return nil
}

func (m *Memory) SpaceByID(_ context.Context, id string) (*model.Space, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.spaces[id]; ok {
		c := *s
		return &c, nil
	}
	return nil, ErrNotFound
}

func (m *Memory) SaveSpace(_ context.Context, s *model.Space) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *s
	m.spaces[s.ID] = &c
	return nil
}

func (m *Memory) MembershipForUser(_ context.Context, userID string) (*model.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mm := range m.memberships {
		if mm.UserID == userID {
			c := *mm
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) MembersOfSpace(_ context.Context, spaceID string) ([]model.Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Membership
	for _, mm := range m.memberships {
		if mm.SpaceID == spaceID {
			out = append(out, *mm)
		}
	}
	return out, nil
}

func (m *Memory) SaveMembership(_ context.Context, mm *model.Membership) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.memberships {
		if x.SpaceID == mm.SpaceID && x.UserID == mm.UserID && x.ID != mm.ID {
			return ErrConflict
		}
	}
	c := *mm
	m.memberships[mm.ID] = &c
	return nil
}

func (m *Memory) GetEntity(_ context.Context, space, typ, id string) (*model.Entity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entities[ekey(space, typ, id)]; ok {
		return e.Clone(), nil
	}
	return nil, ErrNotFound
}

func (m *Memory) ListEntities(_ context.Context, space, typ string, q ListQuery) ([]model.Entity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Entity
	for _, e := range m.entities {
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

func (m *Memory) SaveEntity(_ context.Context, e *model.Entity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := ekey(e.SpaceID, e.Type, e.ID)
	cur, exists := m.entities[k]
	switch {
	case e.Version == 1 && exists:
		return ErrConflict
	case e.Version > 1 && (!exists || cur.Version != e.Version-1):
		return ErrConflict
	}
	m.seq[e.SpaceID]++
	e.Seq = m.seq[e.SpaceID]
	m.entities[k] = e.Clone()
	return nil
}

func (m *Memory) Changes(_ context.Context, space string, after int64, limit int) ([]model.Entity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Entity
	for _, e := range m.entities {
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

func (m *Memory) GetOp(_ context.Context, id string) (*model.OpRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.ops[id]; ok {
		c := *r
		if r.Entity != nil {
			c.Entity = r.Entity.Clone()
		}
		return &c, nil
	}
	return nil, ErrNotFound
}

func (m *Memory) SaveOp(_ context.Context, r *model.OpRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.ops[r.OperationID]; ok {
		return ErrConflict
	}
	c := *r
	if r.Entity != nil {
		c.Entity = r.Entity.Clone()
	}
	m.ops[r.OperationID] = &c
	return nil
}

func (m *Memory) SaveOTP(_ context.Context, c *model.OTPChallenge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	x := *c
	m.otps[strings.ToLower(c.Email)] = &x
	return nil
}

func (m *Memory) GetOTP(_ context.Context, email string) (*model.OTPChallenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.otps[strings.ToLower(email)]; ok {
		x := *c
		return &x, nil
	}
	return nil, ErrNotFound
}

func (m *Memory) AddOTPAttempt(_ context.Context, email string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.otps[strings.ToLower(email)]
	if !ok {
		return 0, ErrNotFound
	}
	c.Attempts++
	return c.Attempts, nil
}

func (m *Memory) DeleteOTP(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.otps, strings.ToLower(email))
	return nil
}

func (m *Memory) SaveSession(_ context.Context, s *model.RefreshSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *s
	m.sessions[s.ID] = &c
	return nil
}

func (m *Memory) SessionByHash(_ context.Context, hash string) (*model.RefreshSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.TokenHash == hash {
			c := *s
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) MarkSessionUsed(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || s.UsedAt != nil || s.RevokedAt != nil {
		return false, nil
	}
	now := nowUTC()
	s.UsedAt = &now
	return true, nil
}

func (m *Memory) RevokeFamily(_ context.Context, family string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := nowUTC()
	for _, s := range m.sessions {
		if s.FamilyID == family && s.RevokedAt == nil {
			t := now
			s.RevokedAt = &t
		}
	}
	return nil
}
