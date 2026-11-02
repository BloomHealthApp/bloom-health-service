package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/chatrole"
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
	s.DeletedAt, s.UpdatedAt = &at, at
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

type chatMessageRepo struct {
	mu       sync.Mutex
	messages map[string]*dao.ChatMessage
}

func NewChatMessageRepo() mongorepo.IChatMessageRepo {
	return &chatMessageRepo{messages: map[string]*dao.ChatMessage{}}
}

func cloneChatMessage(m *dao.ChatMessage) *dao.ChatMessage {
	c := *m
	if m.LockedUntil != nil {
		t := *m.LockedUntil
		c.LockedUntil = &t
	}
	if m.Usage != nil {
		u := *m.Usage
		c.Usage = &u
	}
	return &c
}

func (r *chatMessageRepo) Insert(_ context.Context, msgs ...*dao.ChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range msgs {
		if _, ok := r.messages[m.ID]; ok {
			return apperrors.ErrRepoConflict
		}
		for _, x := range r.messages {
			if x.SessionID == m.SessionID && x.Idx == m.Idx {
				return apperrors.ErrRepoConflict
			}
		}
		r.messages[m.ID] = cloneChatMessage(m)
	}
	return nil
}

func (r *chatMessageRepo) ListForSession(_ context.Context, sessionID string, beforeIdx, limit int) ([]dao.ChatMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []dao.ChatMessage{}
	for _, m := range r.messages {
		if m.SessionID == sessionID && (beforeIdx <= 0 || m.Idx < beforeIdx) {
			out = append(out, *cloneChatMessage(m))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Idx > out[j].Idx })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *chatMessageRepo) CountUserMessagesSince(_ context.Context, userID string, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, m := range r.messages {
		if m.UserID == userID && m.Role == chatrole.User && !m.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

// SettleForTest updates a stored message the way bloom-health-task-worker would (tests only).
func SettleForTest(repo mongorepo.IChatMessageRepo, id string, apply func(m *dao.ChatMessage)) bool {
	r, ok := repo.(*chatMessageRepo)
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.messages[id]
	if ok {
		apply(m)
	}
	return ok
}
