package memory

import (
	"context"
	"sync"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type spaceRepo struct {
	mu     sync.Mutex
	spaces map[string]*dao.Space
}

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
