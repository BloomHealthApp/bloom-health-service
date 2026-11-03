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

type chatSessionRepo struct {
	mu       sync.Mutex
	sessions map[string]*dao.ChatSession
}

func NewChatSessionRepo() mongorepo.IChatSessionRepo {
	return &chatSessionRepo{sessions: map[string]*dao.ChatSession{}}
}

func cloneChatSession(s *dao.ChatSession) *dao.ChatSession {
	c := *s
	c.ContextScopes = append([]string{}, s.ContextScopes...)
	if s.ActiveTurnID != nil {
		t := *s.ActiveTurnID
		c.ActiveTurnID = &t
	}
	if s.DeletedAt != nil {
		t := *s.DeletedAt
		c.DeletedAt = &t
	}
	return &c
}

func (r *chatSessionRepo) live(userID, id string) (*dao.ChatSession, error) {
	s, ok := r.sessions[id]
	if !ok || s.UserID != userID || s.DeletedAt != nil {
		return nil, apperrors.ErrRepoNotFound
	}
	return s, nil
}

func (r *chatSessionRepo) Create(_ context.Context, s *dao.ChatSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[s.ID]; ok {
		return apperrors.ErrRepoConflict
	}
	r.sessions[s.ID] = cloneChatSession(s)
	return nil
}

func (r *chatSessionRepo) Get(_ context.Context, userID, id string) (*dao.ChatSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, err := r.live(userID, id)
	if err != nil {
		return nil, err
	}
	return cloneChatSession(s), nil
}

func (r *chatSessionRepo) ListForUser(_ context.Context, userID string, limit int) ([]dao.ChatSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []dao.ChatSession{}
	for _, s := range r.sessions {
		if s.UserID == userID && s.DeletedAt == nil {
			out = append(out, *cloneChatSession(s))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *chatSessionRepo) SoftDelete(_ context.Context, userID, id string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, err := r.live(userID, id)
	if err != nil {
		return err
	}
	s.DeletedAt, s.UpdatedAt, s.Title = &at, at, ""
	s.Version++
	return nil
}

func (r *chatSessionRepo) BeginTurn(_ context.Context, userID, id, turnID string, at time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, err := r.live(userID, id)
	if err != nil {
		return 0, err
	}
	if s.ActiveTurnID != nil {
		return 0, apperrors.ErrRepoConflict
	}
	first := s.NextIdx
	s.ActiveTurnID, s.UpdatedAt = &turnID, at
	s.NextIdx += 2
	s.Version++
	return first, nil
}

func (r *chatSessionRepo) EndTurn(_ context.Context, id, turnID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[id]; ok && s.ActiveTurnID != nil && *s.ActiveTurnID == turnID {
		s.ActiveTurnID = nil
		s.Version++
	}
	return nil
}

func (r *chatSessionRepo) SetTitle(_ context.Context, userID, id, title string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, err := r.live(userID, id)
	if err != nil {
		return err
	}
	s.Title = title
	s.Version++
	return nil
}

// ChatSessionTombstoneForTest runs inspect on a session even after it was deleted (tests only).
func ChatSessionTombstoneForTest(repo mongorepo.IChatSessionRepo, id string, inspect func(s *dao.ChatSession)) bool {
	r, ok := repo.(*chatSessionRepo)
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if ok {
		inspect(cloneChatSession(s))
	}
	return ok
}
