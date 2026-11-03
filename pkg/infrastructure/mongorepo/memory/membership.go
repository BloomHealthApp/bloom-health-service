package memory

import (
	"context"
	"sync"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type membershipRepo struct {
	mu sync.Mutex
	m  map[string]*dao.Membership
}

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
