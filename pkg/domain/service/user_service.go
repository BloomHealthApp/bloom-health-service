package service

import (
	"context"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// IUserService manages the caller's own profile (allowed for both roles).
type IUserService interface {
	GetMe(ctx context.Context, actor *serviceBos.Actor) (*response.UserResponse, apperrors.IHttpError)
	PatchMe(ctx context.Context, actor *serviceBos.Actor, req *request.PatchMeReq) (*response.UserResponse, apperrors.IHttpError)
	// SetPhone saves an E.164 phone the caller has already confirmed through IAuthService.ConfirmPhoneChange.
	// 409 already_registered when another account took it in the meantime.
	SetPhone(ctx context.Context, actor *serviceBos.Actor, phone string) (*response.UserResponse, apperrors.IHttpError)
}

type userService struct {
	log         logger.ILogger
	users       mongorepo.IUserRepo
	memberships mongorepo.IMembershipRepo
	clock       clock.Clock
}

func NewUserService(log logger.ILogger, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo, clk clock.Clock) IUserService {
	return &userService{log: log, users: users, memberships: memberships, clock: clk}
}

func (s *userService) GetMe(ctx context.Context, actor *serviceBos.Actor) (*response.UserResponse, apperrors.IHttpError) {
	r := buildUserResponse(ctx, s.users, s.memberships, actor.User, actor.Membership)
	return &r, nil
}

func (s *userService) PatchMe(ctx context.Context, actor *serviceBos.Actor, req *request.PatchMeReq) (*response.UserResponse, apperrors.IHttpError) {
	u := *actor.User
	if req.DisplayName.Set {
		u.DisplayName = req.DisplayName.Value
	}
	if req.Avatar.Set {
		if req.Avatar.Null {
			u.Avatar = nil
		} else {
			v := req.Avatar.Value
			u.Avatar = &v
		}
	}
	if req.Gender.Set {
		if req.Gender.Null {
			u.Gender = nil
		} else {
			v := req.Gender.Value
			u.Gender = &v
		}
	}
	if req.PeriodTrackerEnabled.Set {
		if req.PeriodTrackerEnabled.Null {
			u.PeriodTracker = nil
		} else {
			v := req.PeriodTrackerEnabled.Value
			u.PeriodTracker = &v
		}
	}
	if req.Timezone.Set {
		u.Timezone = req.Timezone.Value
	}
	if req.Theme.Set {
		u.Theme = req.Theme.Value
	}
	u.UpdatedAt = s.clock.Now().UTC()
	if err := s.users.Save(ctx, &u); err != nil {
		if apperrors.IsConflict(err) { // unique-index race on phone
			return nil, apperrors.AlreadyRegisteredError("an account with this phone already exists")
		}
		return nil, internalErr(s.log, ctx, "profile save failed", err)
	}
	r := buildUserResponse(ctx, s.users, s.memberships, &u, actor.Membership)
	return &r, nil
}

func (s *userService) SetPhone(ctx context.Context, actor *serviceBos.Actor, phone string) (*response.UserResponse, apperrors.IHttpError) {
	taken := apperrors.AlreadyRegisteredError("an account with this phone already exists")
	if other, err := s.users.ByPhone(ctx, phone); err == nil && other.ID != actor.User.ID {
		return nil, taken
	} else if err != nil && !apperrors.IsNotFound(err) {
		return nil, internalErr(s.log, ctx, "profile phone lookup failed", err)
	}
	u := *actor.User
	u.Phone = &phone
	u.UpdatedAt = s.clock.Now().UTC()
	if err := s.users.Save(ctx, &u); err != nil {
		if apperrors.IsConflict(err) { // unique-index race on phone
			return nil, taken
		}
		return nil, internalErr(s.log, ctx, "profile save failed", err)
	}
	r := buildUserResponse(ctx, s.users, s.memberships, &u, actor.Membership)
	return &r, nil
}
