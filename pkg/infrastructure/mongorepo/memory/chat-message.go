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

func (r *chatMessageRepo) DeleteForSession(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, m := range r.messages {
		if m.SessionID == sessionID {
			delete(r.messages, k)
		}
	}
	return nil
}
