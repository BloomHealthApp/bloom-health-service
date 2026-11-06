package memory

import (
	"context"
	"sync"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type sessionRepo struct {
	mu       sync.Mutex
	sessions map[string]*dao.RefreshSession
}

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

func (r *sessionRepo) RevokeAllForUser(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, s := range r.sessions {
		if s.UserID == userID && s.RevokedAt == nil {
			t := now
			s.RevokedAt = &t
		}
	}
	return nil
}
