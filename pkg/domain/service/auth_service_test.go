package service

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/transport/smtp"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

const testSecret = "0123456789abcdef0123456789abcdef-unit"

type authMocks struct {
	users       *mongorepo.MockIUserRepo
	memberships *mongorepo.MockIMembershipRepo
	otps        *mongorepo.MockIOTPRepo
	sessions    *mongorepo.MockISessionRepo
	mailer      *smtp.CaptureMailer
	provision   *MockIProvisionService
	clock       *clock.Fake
}

// createAuthService builds the service over gomock repositories; setup registers expectations.
func createAuthService(ctrl *gomock.Controller, setup func(m *authMocks)) (IAuthService, *authMocks) {
	m := &authMocks{
		users: mongorepo.NewMockIUserRepo(ctrl), memberships: mongorepo.NewMockIMembershipRepo(ctrl),
		otps: mongorepo.NewMockIOTPRepo(ctrl), sessions: mongorepo.NewMockISessionRepo(ctrl),
		mailer: &smtp.CaptureMailer{}, provision: NewMockIProvisionService(ctrl), clock: clock.NewFake(),
	}
	if setup != nil {
		setup(m)
	}
	cfg := &config.Store{AccessSecret: testSecret, Issuer: "unit", OTPResendAfter: 30 * time.Second}
	return NewAuthService(logger.NewDiscard(), cfg, m.users, m.memberships, m.otps, m.sessions, m.mailer, m.provision, m.clock), m
}

var (
	anu        = &dao.User{ID: "u-anu", Email: "anu@example.com", DisplayName: "Anu", Timezone: "UTC", Theme: "cottonCandy"}
	anuMember  = &dao.Membership{ID: "m1", SpaceID: "space1", UserID: "u-anu", Role: userrole.Owner}
	codeRegexp = regexp.MustCompile(`\b(\d{6})\b`)
)

// expectUserResponse covers the lookups buildUserResponse performs.
func expectUserResponse(m *authMocks) {
	m.memberships.EXPECT().MembersOfSpace(gomock.Any(), "space1").Return([]dao.Membership{*anuMember}, nil).AnyTimes()
	m.users.EXPECT().ByID(gomock.Any(), "u-anu").Return(anu, nil).AnyTimes()
}

var (
	anuPhone = "+919876543210"
	anuFull  = &dao.User{ID: "u-anu", Email: "anu@example.com", Phone: &anuPhone, DisplayName: "Anu", Timezone: "UTC", Theme: "cottonCandy"}
)

func requireStatus(t *testing.T, herr apperrors.IHttpError, status int, code string) {
	t.Helper()
	require.NotNil(t, herr)
	got, _ := herr.Get()
	require.Equal(t, status, got)
	require.Equal(t, code, herr.GetCode())
}

// expectNewChallenge registers the repo calls of issuing a challenge and captures what was saved.
func expectNewChallenge(m *authMocks, saved *dao.OTPChallenge) {
	m.otps.EXPECT().LatestByEmail(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrRepoNotFound).AnyTimes()
	m.otps.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c *dao.OTPChallenge) error {
		*saved = *c
		return nil
	})
}

func TestRequestSignUp(t *testing.T) {
	ctx := context.Background()
	req := func() *request.SignUpReq {
		return &request.SignUpReq{Name: "Anu <b>", Phone: "+919876543210", Email: "anu@example.com"}
	}
	t.Run("stores a signup challenge with pending data, mails the code, creates no account", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var saved dao.OTPChallenge
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "anu@example.com").Return(nil, apperrors.ErrRepoNotFound)
			m.users.EXPECT().ByPhone(gomock.Any(), "+919876543210").Return(nil, apperrors.ErrRepoNotFound)
			expectNewChallenge(m, &saved)
			// no provision / users.Save expectations: gomock fails the test if an account is created
		})
		resp, herr := svc.RequestSignUp(ctx, req())
		require.Nil(t, herr)
		require.Equal(t, "sent", resp.Status)
		require.Equal(t, saved.ChallengeID, resp.ChallengeID)
		require.Equal(t, dao.OTPPurposeSignUp, saved.Purpose)
		require.Equal(t, "anu@example.com", saved.Email)
		require.Equal(t, "Anu <b>", saved.PendingName)
		require.Equal(t, "+919876543210", saved.PendingPhone)
		require.WithinDuration(t, m.clock.Now().Add(10*time.Minute), saved.ExpiresAt, 5*time.Second)
		msgs := m.mailer.Snapshot()
		require.Len(t, msgs, 1)
		require.Equal(t, "anu@example.com", msgs[0].To)
		code := codeRegexp.FindString(msgs[0].Body)
		require.Len(t, code, 6)
		require.NotContains(t, saved.CodeHash, code, "only a hash is stored")
		require.Contains(t, msgs[0].HTML, "Welcome, Anu &lt;b&gt;!", "name is HTML-escaped")
	})
	t.Run("email already registered", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "anu@example.com").Return(anuFull, nil)
		})
		_, herr := svc.RequestSignUp(ctx, req())
		requireStatus(t, herr, 409, "already_registered")
		require.Empty(t, m.mailer.Snapshot())
	})
	t.Run("phone already registered", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "anu@example.com").Return(nil, apperrors.ErrRepoNotFound)
			m.users.EXPECT().ByPhone(gomock.Any(), "+919876543210").Return(anuFull, nil)
		})
		_, herr := svc.RequestSignUp(ctx, req())
		requireStatus(t, herr, 409, "already_registered")
		require.Empty(t, m.mailer.Snapshot())
	})
	t.Run("resend cooldown is 429 with Retry-After", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrRepoNotFound)
			m.users.EXPECT().ByPhone(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrRepoNotFound)
			m.otps.EXPECT().LatestByEmail(gomock.Any(), "anu@example.com").Return(&dao.OTPChallenge{ChallengeID: "c1", CreatedAt: m.clock.Now().Add(-5 * time.Second)}, nil)
		})
		_, herr := svc.RequestSignUp(ctx, req())
		requireStatus(t, herr, 429, "rate_limited")
		require.NotEmpty(t, herr.GetHeaders()["Retry-After"])
		require.Empty(t, m.mailer.Snapshot())
	})
	t.Run("mail failure is a 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var saved dao.OTPChallenge
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrRepoNotFound)
			m.users.EXPECT().ByPhone(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrRepoNotFound)
			expectNewChallenge(m, &saved)
		})
		m.mailer.Err = errors.New("smtp down")
		_, herr := svc.RequestSignUp(ctx, req())
		requireStatus(t, herr, 500, "internal")
	})
}

func TestRequestSignIn(t *testing.T) {
	ctx := context.Background()
	t.Run("by email mails the code to that account", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var saved dao.OTPChallenge
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "anu@example.com").Return(anuFull, nil)
			expectNewChallenge(m, &saved)
		})
		resp, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: "anu@example.com", IsEmail: true})
		require.Nil(t, herr)
		require.Equal(t, saved.ChallengeID, resp.ChallengeID)
		require.Equal(t, dao.OTPPurposeSignIn, saved.Purpose)
		msgs := m.mailer.Snapshot()
		require.Len(t, msgs, 1)
		require.Equal(t, "anu@example.com", msgs[0].To)
		require.Contains(t, msgs[0].HTML, "Welcome back")
		require.NotContains(t, saved.CodeHash, codeRegexp.FindString(msgs[0].Body))
	})
	t.Run("by phone mails the mapped email, not the phone", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var saved dao.OTPChallenge
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByPhone(gomock.Any(), anuPhone).Return(anuFull, nil)
			expectNewChallenge(m, &saved)
		})
		_, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: anuPhone})
		require.Nil(t, herr)
		msgs := m.mailer.Snapshot()
		require.Len(t, msgs, 1)
		require.Equal(t, "anu@example.com", msgs[0].To)
		require.Equal(t, "anu@example.com", saved.Email)
	})
	t.Run("unknown identifier gets a decoy, nothing stored or sent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByPhone(gomock.Any(), "+10000000000").Return(nil, apperrors.ErrRepoNotFound).Times(2)
			// no otps.Save expectation: storing a challenge would fail the test
		})
		a, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: "+10000000000"})
		require.Nil(t, herr)
		b, _ := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: "+10000000000"})
		require.Equal(t, "sent", a.Status)
		require.NotEmpty(t, a.ChallengeID)
		require.NotEqual(t, a.ChallengeID, b.ChallengeID, "decoys are random")
		require.Empty(t, m.mailer.Snapshot())
	})
	t.Run("cooldown returns the live challenge and sends nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "anu@example.com").Return(anuFull, nil)
			m.otps.EXPECT().LatestByEmail(gomock.Any(), "anu@example.com").Return(
				&dao.OTPChallenge{ChallengeID: "live", Purpose: dao.OTPPurposeSignIn, CreatedAt: m.clock.Now().Add(-5 * time.Second)}, nil)
		})
		resp, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: "anu@example.com", IsEmail: true})
		require.Nil(t, herr)
		require.Equal(t, "live", resp.ChallengeID)
		require.Empty(t, m.mailer.Snapshot())
	})
	t.Run("a new challenge retires the previous one", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "anu@example.com").Return(anuFull, nil)
			m.otps.EXPECT().LatestByEmail(gomock.Any(), "anu@example.com").Return(
				&dao.OTPChallenge{ChallengeID: "old", Purpose: dao.OTPPurposeSignIn, CreatedAt: m.clock.Now().Add(-time.Minute)}, nil).Times(2)
			m.otps.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			m.otps.EXPECT().Delete(gomock.Any(), "old").Return(nil)
		})
		_, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: "anu@example.com", IsEmail: true})
		require.Nil(t, herr)
	})
	t.Run("mail failure looks like success (decoy)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var saved dao.OTPChallenge
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "anu@example.com").Return(anuFull, nil)
			expectNewChallenge(m, &saved)
		})
		m.mailer.Err = errors.New("smtp down")
		resp, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: "anu@example.com", IsEmail: true})
		require.Nil(t, herr)
		require.NotEmpty(t, resp.ChallengeID)
		require.NotEqual(t, saved.ChallengeID, resp.ChallengeID)
	})
}

// issueChallenge runs RequestSignIn for Anu and returns the stored challenge plus the mailed code.
func issueChallenge(t *testing.T, ctrl *gomock.Controller, purpose string) (IAuthService, *authMocks, *dao.OTPChallenge, string) {
	t.Helper()
	var saved dao.OTPChallenge
	svc, m := createAuthService(ctrl, func(m *authMocks) { expectNewChallenge(m, &saved) })
	ctx := context.Background()
	if purpose == dao.OTPPurposeSignUp {
		m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound)
		m.users.EXPECT().ByPhone(gomock.Any(), anuPhone).Return(nil, apperrors.ErrRepoNotFound)
		_, herr := svc.RequestSignUp(ctx, &request.SignUpReq{Name: "Anu", Phone: anuPhone, Email: anu.Email})
		require.Nil(t, herr)
	} else {
		m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anuFull, nil)
		_, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: anu.Email, IsEmail: true})
		require.Nil(t, herr)
	}
	return svc, m, &saved, codeRegexp.FindString(m.mailer.Snapshot()[0].Body)
}

func TestVerifyOTP(t *testing.T) {
	ctx := context.Background()
	t.Run("signin: correct code is single use and issues tokens", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil) // legacy user: no phone
		var sess *dao.RefreshSession
		m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s *dao.RefreshSession) error {
			sess = s
			return nil
		})
		m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil)
		expectUserResponse(m)
		resp, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: code})
		require.Nil(t, herr)
		require.Equal(t, 900, resp.ExpiresIn)
		require.Equal(t, "owner", resp.User.Role)
		require.Nil(t, resp.User.Phone, "legacy account has a null phone")
		require.NotEqual(t, resp.RefreshToken, sess.TokenHash, "refresh token is stored hashed")
		require.Equal(t, sha(resp.RefreshToken), sess.TokenHash)
	})
	t.Run("signup: creates the account from the pending data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignUp)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		m.provision.EXPECT().EnsureAccount(gomock.Any(), "Anu", anuPhone, gomock.Any(), gomock.Any(), anu.Email, gomock.Any()).Return(anuFull, nil)
		m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil)
		expectUserResponse(m)
		resp, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: code})
		require.Nil(t, herr)
		require.Equal(t, anuPhone, *resp.User.Phone)
	})
	t.Run("signup: unique-index race maps to 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignUp)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		m.provision.EXPECT().EnsureAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrRepoConflict)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: code})
		requireStatus(t, herr, 409, "already_registered")
	})
	t.Run("unknown challenge id is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createAuthService(ctrl, func(m *authMocks) {
			m.otps.EXPECT().Get(gomock.Any(), "decoy").Return(nil, apperrors.ErrRepoNotFound)
		})
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: "decoy", Code: "123456"})
		requireStatus(t, herr, 401, "unauthorized")
		_, herr = svc.VerifyOTP(ctx, &request.OTPVerifyReq{Code: "123456"})
		requireStatus(t, herr, 401, "unauthorized")
	})
	t.Run("wrong code counts an attempt and is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, _ := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().AddAttempt(gomock.Any(), ch.ChallengeID).Return(1, nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: "000000"})
		requireStatus(t, herr, 401, "unauthorized")
	})
	t.Run("code is bound to its challenge", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		other := *ch
		other.ChallengeID = "other-challenge"
		m.otps.EXPECT().Get(gomock.Any(), "other-challenge").Return(&other, nil)
		m.otps.EXPECT().AddAttempt(gomock.Any(), "other-challenge").Return(1, nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: "other-challenge", Code: code})
		require.NotNil(t, herr)
	})
	t.Run("fifth wrong attempt burns the challenge", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, _ := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().AddAttempt(gomock.Any(), ch.ChallengeID).Return(5, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: "000000"})
		require.NotNil(t, herr)
	})
	t.Run("challenge already at the attempt limit is deleted and rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		ch.Attempts = 5
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: code})
		require.NotNil(t, herr)
	})
	t.Run("expired challenge is deleted and rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		m.clock.SetOffset(11 * time.Minute)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: code})
		require.NotNil(t, herr)
	})
	t.Run("signin for an account deleted meanwhile is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: code})
		requireStatus(t, herr, 401, "unauthorized")
	})
	t.Run("user without membership gets 403 after verification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueChallenge(t, ctrl, dao.OTPPurposeSignIn)
		m.otps.EXPECT().Get(gomock.Any(), ch.ChallengeID).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), ch.ChallengeID).Return(nil)
		m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil)
		m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(nil, apperrors.ErrRepoNotFound)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: ch.ChallengeID, Code: code})
		requireStatus(t, herr, 403, "forbidden")
	})
}

func TestRefreshRotation(t *testing.T) {
	ctx := context.Background()
	live := func() *dao.RefreshSession {
		return &dao.RefreshSession{ID: "s1", FamilyID: "fam1", UserID: anu.ID, TokenHash: sha("tok"), ExpiresAt: time.Now().Add(time.Hour)}
	}
	tests := []struct {
		name       string
		session    func() *dao.RefreshSession
		lookupErr  error
		markUsed   *bool // nil: MarkUsed not expected
		wantRevoke bool
		wantStatus int // 0 = success
	}{
		{name: "rotates within the same family", session: live, markUsed: ptr(true)},
		{name: "unknown token", lookupErr: apperrors.ErrRepoNotFound, wantStatus: 401},
		{name: "reuse of a rotated token revokes the family", wantRevoke: true, wantStatus: 401, session: func() *dao.RefreshSession {
			s := live()
			now := time.Now()
			s.UsedAt = &now
			return s
		}},
		{name: "lost race on MarkUsed revokes the family", session: live, markUsed: ptr(false), wantRevoke: true, wantStatus: 401},
		{name: "revoked session", wantStatus: 401, session: func() *dao.RefreshSession {
			s := live()
			now := time.Now()
			s.RevokedAt = &now
			return s
		}},
		{name: "expired session", wantStatus: 401, session: func() *dao.RefreshSession {
			s := live()
			s.ExpiresAt = time.Now().Add(-time.Minute)
			return s
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			var newSess *dao.RefreshSession
			svc, _ := createAuthService(ctrl, func(m *authMocks) {
				if tc.lookupErr != nil {
					m.sessions.EXPECT().ByHash(gomock.Any(), sha("tok")).Return(nil, tc.lookupErr)
				} else {
					m.sessions.EXPECT().ByHash(gomock.Any(), sha("tok")).Return(tc.session(), nil)
				}
				if tc.wantRevoke {
					m.sessions.EXPECT().RevokeFamily(gomock.Any(), "fam1").Return(nil)
				}
				if tc.markUsed != nil {
					m.sessions.EXPECT().MarkUsed(gomock.Any(), "s1").Return(*tc.markUsed, nil)
				}
				if tc.wantStatus == 0 {
					m.users.EXPECT().ByID(gomock.Any(), anu.ID).Return(anu, nil).AnyTimes()
					m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s *dao.RefreshSession) error {
						newSess = s
						return nil
					})
					m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil)
					m.memberships.EXPECT().MembersOfSpace(gomock.Any(), "space1").Return([]dao.Membership{*anuMember}, nil)
				}
			})
			resp, herr := svc.Refresh(ctx, &request.RefreshTokenReq{RefreshToken: "tok"})
			if tc.wantStatus != 0 {
				require.NotNil(t, herr)
				status, _ := herr.Get()
				require.Equal(t, tc.wantStatus, status)
				return
			}
			require.Nil(t, herr)
			require.NotEqual(t, "tok", resp.RefreshToken)
			require.Equal(t, "fam1", newSess.FamilyID, "rotated session stays in the family")
			require.Equal(t, sha(resp.RefreshToken), newSess.TokenHash)
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestLogoutRevokesFamily(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _ := createAuthService(ctrl, func(m *authMocks) {
		m.sessions.EXPECT().ByHash(gomock.Any(), sha("tok")).Return(&dao.RefreshSession{ID: "s1", FamilyID: "fam1"}, nil)
		m.sessions.EXPECT().RevokeFamily(gomock.Any(), "fam1").Return(nil)
	})
	require.Nil(t, svc.Logout(context.Background(), &request.RefreshTokenReq{RefreshToken: "tok"}))
}

func TestAuthenticate(t *testing.T) {
	ctx := context.Background()
	t.Run("role comes from the database membership, not the token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		viewer := &dao.Membership{ID: "m2", SpaceID: "space1", UserID: anu.ID, Role: userrole.Viewer}
		svc, m := createAuthService(ctrl, nil)
		token := mintForTest(t, svc, m)
		m.users.EXPECT().ByID(gomock.Any(), anu.ID).Return(anu, nil)
		m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(viewer, nil)
		actor, herr := svc.Authenticate(ctx, token)
		require.Nil(t, herr)
		require.False(t, actor.IsOwner())
		require.Equal(t, "space1", actor.SpaceID())
	})
	t.Run("expired token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, nil)
		m.clock.SetOffset(-time.Hour)
		token := mintForTest(t, svc, m)
		m.clock.SetOffset(0)
		_, herr := svc.Authenticate(ctx, token)
		require.NotNil(t, herr)
		require.Equal(t, apperrors.CodeTokenExpired, herr.GetCode())
	})
	t.Run("garbage and wrong-signature tokens", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, _ := createAuthService(ctrl, nil)
		for _, tok := range []string{"", "garbage", "a.b.c"} {
			_, herr := svc.Authenticate(ctx, tok)
			require.NotNil(t, herr, tok)
			require.Equal(t, apperrors.CodeUnauthorized, herr.GetCode())
		}
	})
	t.Run("valid token without membership is forbidden", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, nil)
		token := mintForTest(t, svc, m)
		m.users.EXPECT().ByID(gomock.Any(), anu.ID).Return(anu, nil)
		m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(nil, apperrors.ErrRepoNotFound)
		_, herr := svc.Authenticate(ctx, token)
		require.NotNil(t, herr)
		require.Equal(t, apperrors.CodeForbidden, herr.GetCode())
	})
}

// mintForTest obtains a real access token by driving the sign-in flow against the same mocks.
func mintForTest(t *testing.T, svc IAuthService, m *authMocks) string {
	t.Helper()
	ctx := context.Background()
	var ch dao.OTPChallenge
	m.otps.EXPECT().LatestByEmail(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound).AnyTimes()
	m.otps.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, string) (*dao.OTPChallenge, error) {
		c := ch
		return &c, nil
	}).AnyTimes()
	m.otps.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c *dao.OTPChallenge) error {
		ch = *c
		return nil
	}).AnyTimes()
	m.otps.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil).Times(1)
	m.memberships.EXPECT().MembersOfSpace(gomock.Any(), "space1").Return([]dao.Membership{*anuMember}, nil).AnyTimes()
	m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil).Times(2)
	m.users.EXPECT().ByID(gomock.Any(), anu.ID).Return(anu, nil).Times(1)
	resp0, herr := svc.RequestSignIn(ctx, &request.SignInReq{Identifier: anu.Email, IsEmail: true})
	require.Nil(t, herr)
	code := codeRegexp.FindString(m.mailer.Snapshot()[len(m.mailer.Snapshot())-1].Body)
	resp, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: resp0.ChallengeID, Code: code})
	require.Nil(t, herr)
	return resp.AccessToken
}

func TestSignUpAvatarIsStoredInChallengeAndAppliedAtVerify(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		avatar *string
		want   string
	}{"with avatar": {avatarPtr("bubu-04"), "bubu-04"}, "without avatar": {nil, ""}} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			var saved dao.OTPChallenge
			svc, m := createAuthService(ctrl, func(m *authMocks) {
				m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound)
				m.users.EXPECT().ByPhone(gomock.Any(), anuPhone).Return(nil, apperrors.ErrRepoNotFound)
				expectNewChallenge(m, &saved)
			})
			_, herr := svc.RequestSignUp(ctx, &request.SignUpReq{Name: "Anu", Phone: anuPhone, Email: anu.Email, Avatar: tc.avatar})
			require.Nil(t, herr)
			require.Equal(t, tc.want, saved.PendingAvatar)

			code := codeRegexp.FindString(m.mailer.Snapshot()[0].Body)
			m.otps.EXPECT().Get(gomock.Any(), saved.ChallengeID).Return(&saved, nil)
			m.otps.EXPECT().Delete(gomock.Any(), saved.ChallengeID).Return(nil)
			m.provision.EXPECT().EnsureAccount(gomock.Any(), "Anu", anuPhone, tc.want, gomock.Any(), anu.Email, gomock.Any()).Return(anuFull, nil)
			m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil)
			expectUserResponse(m)
			_, herr = svc.VerifyOTP(ctx, &request.OTPVerifyReq{ChallengeID: saved.ChallengeID, Code: code})
			require.Nil(t, herr)
		})
	}
}

func avatarPtr(s string) *string { return &s }

func TestRequestSignInLogsPhoneMissWithoutTheNumber(t *testing.T) {
	ctx := context.Background()
	run := func(t *testing.T, req *request.SignInReq, expect func(*authMocks)) string {
		ctrl := gomock.NewController(t)
		_, m := createAuthService(ctrl, expect)
		var out bytes.Buffer
		cfg := &config.Store{AccessSecret: testSecret, Issuer: "unit", OTPResendAfter: 30 * time.Second}
		svc := NewAuthService(logger.NewWithWriter(&out, "info"), cfg, m.users, m.memberships, m.otps, m.sessions, m.mailer, m.provision, m.clock)
		resp, herr := svc.RequestSignIn(ctx, req)
		require.Nil(t, herr)
		require.Equal(t, "sent", resp.Status)
		require.Empty(t, m.mailer.Snapshot())
		return out.String()
	}
	t.Run("unknown phone", func(t *testing.T) {
		logs := run(t, &request.SignInReq{Identifier: "+919876500000"}, func(m *authMocks) {
			m.users.EXPECT().ByPhone(gomock.Any(), "+919876500000").Return(nil, apperrors.ErrRepoNotFound)
		})
		require.Contains(t, logs, "signin: no account for phone")
		require.NotContains(t, logs, "9876500000")
	})
	t.Run("unknown email stays silent", func(t *testing.T) {
		logs := run(t, &request.SignInReq{Identifier: "nobody@example.com", IsEmail: true}, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "nobody@example.com").Return(nil, apperrors.ErrRepoNotFound)
		})
		require.NotContains(t, logs, "no account")
	})
	t.Run("rate-limited decoy isn't counted as a miss", func(t *testing.T) {
		logs := run(t, &request.SignInReq{Identifier: "decoy|abc"}, func(m *authMocks) {
			m.users.EXPECT().ByPhone(gomock.Any(), "decoy|abc").Return(nil, apperrors.ErrRepoNotFound)
		})
		require.NotContains(t, logs, "no account")
	})
}
