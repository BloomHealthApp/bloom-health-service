package service

import (
	"context"

	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/dto/response"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils/clock"
)

// IUserService manages the caller's own profile (allowed for both roles).
type IUserService interface {
	GetMe(ctx context.Context, actor *serviceBos.Actor) (*response.UserResponse, apperrors.IHttpError)
	PatchMe(ctx context.Context, actor *serviceBos.Actor, req *request.PatchMeReq) (*response.UserResponse, apperrors.IHttpError)
}

type userService struct {
	log         logger.ILogger
	users       mongorepo.IUserRepo
	memberships mongorepo.IMembershipRepo
	clock       clock.Clock
}

// NewUserService wires the user service.
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
	if req.Timezone.Set {
		u.Timezone = req.Timezone.Value
	}
	if req.Theme.Set {
		u.Theme = req.Theme.Value
	}
	u.UpdatedAt = s.clock.Now().UTC()
	if err := s.users.Save(ctx, &u); err != nil {
		return nil, internalErr(s.log, ctx, "profile save failed", err)
	}
	r := buildUserResponse(ctx, s.users, s.memberships, &u, actor.Membership)
	return &r, nil
}
