package service

import (
	"context"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/transport/smtp"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// IUserService manages the caller's own profile (allowed for both roles).
type IUserService interface {
	GetMe(ctx context.Context, actor *bo.Actor) (*response.UserResponse, apperrors.IHttpError)
	PatchMe(ctx context.Context, actor *bo.Actor, req *request.PatchMeReq) (*response.UserResponse, apperrors.IHttpError)
	// SetPhone saves an E.164 phone the caller has already confirmed through IAuthService.ConfirmPhoneChange.
	// 409 already_registered when another account took it in the meantime.
	SetPhone(ctx context.Context, actor *bo.Actor, phone string) (*response.UserResponse, apperrors.IHttpError)
	// DeleteMe schedules the caller's account for deletion: it is locked at once, every device is signed out, and it
	// is erased after constants.AccountDeletionGrace unless restored. Calling it again returns the same date.
	DeleteMe(ctx context.Context, actor *bo.Actor) (*response.DeletionScheduledResponse, apperrors.IHttpError)
	// RestoreMe cancels a scheduled deletion while the grace period lasts.
	RestoreMe(ctx context.Context, actor *bo.Actor) (*response.UserResponse, apperrors.IHttpError)
	// PurgeDue erases every account whose grace period has ended and returns how many were erased. For an owner
	// that includes the whole space, so any viewers lose access to it.
	PurgeDue(ctx context.Context) (int, apperrors.IHttpError)
}

type userService struct {
	log         logger.ILogger
	users       mongorepo.IUserRepo
	memberships mongorepo.IMembershipRepo
	accounts    mongorepo.IAccountRepo
	sessions    mongorepo.ISessionRepo
	mailer      smtp.IMailer
	clock       clock.Clock
}

func NewUserService(log logger.ILogger, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo, accounts mongorepo.IAccountRepo,
	sessions mongorepo.ISessionRepo, mailer smtp.IMailer, clk clock.Clock) IUserService {
	return &userService{log: log, users: users, memberships: memberships, accounts: accounts, sessions: sessions, mailer: mailer, clock: clk}
}

func (s *userService) GetMe(ctx context.Context, actor *bo.Actor) (*response.UserResponse, apperrors.IHttpError) {
	r := buildUserResponse(ctx, s.users, s.memberships, actor.User, actor.Membership)
	return &r, nil
}

func (s *userService) PatchMe(ctx context.Context, actor *bo.Actor, req *request.PatchMeReq) (*response.UserResponse, apperrors.IHttpError) {
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

func (s *userService) SetPhone(ctx context.Context, actor *bo.Actor, phone string) (*response.UserResponse, apperrors.IHttpError) {
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

func (s *userService) DeleteMe(ctx context.Context, actor *bo.Actor) (*response.DeletionScheduledResponse, apperrors.IHttpError) {
	u := *actor.User
	if u.DeleteAfter == nil {
		now := s.clock.Now().UTC()
		after := now.Add(constants.AccountDeletionGrace)
		u.DeletionRequestedAt, u.DeleteAfter, u.UpdatedAt = &now, &after, now
		if err := s.users.Save(ctx, &u); err != nil {
			return nil, internalErr(s.log, ctx, "account deletion: save failed", err)
		}
		s.log.Info(ctx, "account deletion scheduled", map[string]any{"userId": u.ID, "owner": actor.IsOwner()})
		s.mail(ctx, &u, smtp.AccountDeletionScheduled)
	}
	if err := s.sessions.RevokeAllForUser(ctx, u.ID); err != nil {
		return nil, internalErr(s.log, ctx, "account deletion: session revoke failed", err)
	}
	return &response.DeletionScheduledResponse{DeleteAfter: u.DeleteAfter.UTC().Format(time.RFC3339)}, nil
}

func (s *userService) RestoreMe(ctx context.Context, actor *bo.Actor) (*response.UserResponse, apperrors.IHttpError) {
	u := *actor.User
	if u.DeleteAfter == nil {
		return nil, apperrors.ConflictError("this account is not scheduled for deletion", nil)
	}
	if overdue(&u, s.clock.Now()) {
		return nil, apperrors.UnauthorizedError("this account has been deleted")
	}
	u.DeletionRequestedAt, u.DeleteAfter, u.UpdatedAt = nil, nil, s.clock.Now().UTC()
	if err := s.users.Save(ctx, &u); err != nil {
		return nil, internalErr(s.log, ctx, "account restore: save failed", err)
	}
	s.log.Info(ctx, "account restored", map[string]any{"userId": u.ID})
	s.mail(ctx, &u, smtp.AccountRestored)
	r := buildUserResponse(ctx, s.users, s.memberships, &u, actor.Membership)
	return &r, nil
}

func (s *userService) PurgeDue(ctx context.Context) (int, apperrors.IHttpError) {
	erased := 0
	for {
		due, err := s.users.DueForDeletion(ctx, s.clock.Now().UTC(), constants.PurgeBatch)
		if err != nil {
			return erased, internalErr(s.log, ctx, "account purge: lookup failed", err)
		}
		for i := range due {
			if err := eraseAccount(ctx, s.accounts, s.memberships, &due[i]); err != nil {
				return erased, internalErr(s.log, ctx, "account purge: erase failed", err)
			}
			erased++
			s.mail(ctx, &due[i], smtp.AccountDeleted)
		}
		if len(due) < constants.PurgeBatch {
			s.log.Info(ctx, "accounts purged", map[string]any{"count": erased})
			return erased, nil
		}
	}
}

// mail sends an account email. A failure is logged and never fails the request: the change has already happened.
func (s *userService) mail(ctx context.Context, u *dao.User, kind smtp.AccountKind) {
	subject, text, html := smtp.AccountEmail(kind, deletionDate(u))
	if err := s.mailer.Send(ctx, u.Email, subject, text, html); err != nil {
		s.log.Error(ctx, "account email failed", map[string]any{"err": err.Error()})
	}
}

// deletionDate is the day the account is erased, in the user's timezone, as "21 November 2026".
func deletionDate(u *dao.User) string {
	if u.DeleteAfter == nil {
		return ""
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return u.DeleteAfter.In(loc).Format("2 January 2006")
}
