package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type connectionRepo struct {
	mu    sync.Mutex
	conns map[string]*dao.Connection
}

func NewConnectionRepo() mongorepo.IConnectionRepo {
	return &connectionRepo{conns: map[string]*dao.Connection{}}
}

func cloneConnection(c *dao.Connection) *dao.Connection {
	out := *c
	if c.RespondedAt != nil {
		t := *c.RespondedAt
		out.RespondedAt = &t
	}
	if c.ExpiresAt != nil {
		t := *c.ExpiresAt
		out.ExpiresAt = &t
	}
	return &out
}

func (r *connectionRepo) Create(_ context.Context, c *dao.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.conns {
		if x.PairKey == c.PairKey {
			return apperrors.ErrRepoConflict
		}
	}
	r.conns[c.ID] = cloneConnection(c)
	return nil
}

func (r *connectionRepo) ByID(_ context.Context, id string) (*dao.Connection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.conns[id]; ok {
		return cloneConnection(c), nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *connectionRepo) ByPair(_ context.Context, a, b string) (*dao.Connection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := dao.PairKey(a, b)
	for _, c := range r.conns {
		if c.PairKey == key {
			return cloneConnection(c), nil
		}
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *connectionRepo) ListForUser(_ context.Context, userID string, limit int) ([]dao.Connection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []dao.Connection{}
	for _, c := range r.conns {
		if c.Involves(userID) && c.Status != dao.ConnectionDeclined {
			out = append(out, *cloneConnection(c))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *connectionRepo) respond(id, addresseeID string, apply func(c *dao.Connection)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.conns[id]
	if !ok || c.AddresseeID != addresseeID || c.Status != dao.ConnectionPending {
		return apperrors.ErrRepoNotFound
	}
	apply(c)
	return nil
}

func (r *connectionRepo) Accept(_ context.Context, id, addresseeID string, at time.Time) error {
	return r.respond(id, addresseeID, func(c *dao.Connection) {
		c.Status, c.RespondedAt = dao.ConnectionAccepted, &at
	})
}

func (r *connectionRepo) Decline(_ context.Context, id, addresseeID string, at, until time.Time) error {
	return r.respond(id, addresseeID, func(c *dao.Connection) {
		c.Status, c.RespondedAt, c.ExpiresAt = dao.ConnectionDeclined, &at, &until
	})
}

func (r *connectionRepo) Delete(_ context.Context, id, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.conns[id]
	if !ok || !c.Involves(userID) {
		return apperrors.ErrRepoNotFound
	}
	delete(r.conns, id)
	return nil
}

// eraseUser drops every connection involving userID (account erase).
func (r *connectionRepo) eraseUser(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, c := range r.conns {
		if c.Involves(userID) {
			delete(r.conns, id)
		}
	}
}
