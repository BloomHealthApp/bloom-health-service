package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type otpRepo struct {
	mu   sync.Mutex
	otps map[string]*dao.OTPChallenge
}

func NewOTPRepo() mongorepo.IOTPRepo { return &otpRepo{otps: map[string]*dao.OTPChallenge{}} }

func (r *otpRepo) Save(_ context.Context, c *dao.OTPChallenge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x := *c
	x.Email = strings.ToLower(c.Email)
	r.otps[c.ChallengeID] = &x
	return nil
}

func (r *otpRepo) Get(_ context.Context, challengeID string) (*dao.OTPChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.otps[challengeID]; ok {
		x := *c
		return &x, nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *otpRepo) LatestByEmail(_ context.Context, email string) (*dao.OTPChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var best *dao.OTPChallenge
	for _, c := range r.otps {
		if c.Email == strings.ToLower(email) && (best == nil || c.CreatedAt.After(best.CreatedAt)) {
			best = c
		}
	}
	if best == nil {
		return nil, apperrors.ErrRepoNotFound
	}
	x := *best
	return &x, nil
}

func (r *otpRepo) AddAttempt(_ context.Context, challengeID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.otps[challengeID]
	if !ok {
		return 0, apperrors.ErrRepoNotFound
	}
	c.Attempts++
	return c.Attempts, nil
}

func (r *otpRepo) Delete(_ context.Context, challengeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.otps, challengeID)
	return nil
}
