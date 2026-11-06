package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type userRepo struct {
	mu    sync.Mutex
	users map[string]*dao.User
}

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

func (r *userRepo) DueForDeletion(_ context.Context, now time.Time, limit int) ([]dao.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []dao.User
	for _, u := range r.users {
		if u.DeleteAfter != nil && !u.DeleteAfter.After(now) && len(out) < limit {
			out = append(out, *u)
		}
	}
	return out, nil
}
