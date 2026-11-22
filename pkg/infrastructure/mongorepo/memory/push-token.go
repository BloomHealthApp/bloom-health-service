package memory

import (
	"context"
	"sync"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type pushTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]dao.PushToken
}

func NewPushTokenRepo() mongorepo.IPushTokenRepo {
	return &pushTokenRepo{tokens: map[string]dao.PushToken{}}
}

func (r *pushTokenRepo) Save(_ context.Context, t *dao.PushToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[t.Token] = *t
	return nil
}

func (r *pushTokenRepo) Delete(_ context.Context, token, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[token]; ok && t.UserID == userID {
		delete(r.tokens, token)
	}
	return nil
}

func (r *pushTokenRepo) ForUser(_ context.Context, userID string) ([]dao.PushToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []dao.PushToken{}
	for _, t := range r.tokens {
		if t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *pushTokenRepo) DeleteTokens(_ context.Context, tokens []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tokens {
		delete(r.tokens, t)
	}
	return nil
}

// eraseUser drops every token of userID (account erase).
func (r *pushTokenRepo) eraseUser(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, t := range r.tokens {
		if t.UserID == userID {
			delete(r.tokens, k)
		}
	}
}
