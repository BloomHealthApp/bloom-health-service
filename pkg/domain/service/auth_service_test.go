package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/bubu-bloom/service/config"
	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/enums/userrole"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/infrastructure/transport/smtp"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils/clock"
)

const testSecret = "0123456789abcdef0123456789abcdef-unit"

type authMocks struct {
	users       *mongorepo.MockIUserRepo
	memberships *mongorepo.MockIMembershipRepo
	otps        *mongorepo.MockIOTPRepo
	sessions    *mongorepo.MockISessionRepo
	mailer      *smtp.CaptureMailer
	clock       *clock.Fake
}

// createAuthService builds the service over gomock repositories; setup registers expectations.
func createAuthService(ctrl *gomock.Controller, setup func(m *authMocks)) (IAuthService, *authMocks) {
	m := &authMocks{
		users: mongorepo.NewMockIUserRepo(ctrl), memberships: mongorepo.NewMockIMembershipRepo(ctrl),
		otps: mongorepo.NewMockIOTPRepo(ctrl), sessions: mongorepo.NewMockISessionRepo(ctrl),
		mailer: &smtp.CaptureMailer{}, clock: clock.NewFake(),
	}
	if setup != nil {
		setup(m)
	}
	cfg := &config.Store{AccessSecret: testSecret, Issuer: "unit", OTPResendAfter: 30 * time.Second}
	return NewAuthService(logger.NewDiscard(), cfg, m.users, m.memberships, m.otps, m.sessions, m.mailer, m.clock), m
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

func TestRequestOTP(t *testing.T) {
	ctx := context.Background()
	t.Run("unknown email sends nothing and reveals nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), "stranger@example.com").Return(nil, apperrors.ErrRepoNotFound)
		})
		require.Nil(t, svc.RequestOTP(ctx, &request.OTPRequestReq{Email: "stranger@example.com"}))
		require.Empty(t, m.mailer.Snapshot())
	})
	t.Run("known email stores only a hash and mails the code", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		var saved *dao.OTPChallenge
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil)
			m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound)
			m.otps.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c *dao.OTPChallenge) error {
				saved = c
				return nil
			})
		})
		require.Nil(t, svc.RequestOTP(ctx, &request.OTPRequestReq{Email: anu.Email}))
		msgs := m.mailer.Snapshot()
		require.Len(t, msgs, 1)
		code := codeRegexp.FindString(msgs[0].Body)
		require.Len(t, code, 6)
		require.NotContains(t, saved.CodeHash, code)
		require.WithinDuration(t, m.clock.Now().Add(10*time.Minute), saved.ExpiresAt, 5*time.Second)
	})
	t.Run("resend cooldown silently skips", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m := createAuthService(ctrl, func(m *authMocks) {
			m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil)
			m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(&dao.OTPChallenge{Email: anu.Email, ExpiresAt: m.clock.Now().Add(9*time.Minute + 50*time.Second)}, nil)
		})
		require.Nil(t, svc.RequestOTP(ctx, &request.OTPRequestReq{Email: anu.Email}))
		require.Empty(t, m.mailer.Snapshot())
	})
}

// issueOTP runs RequestOTP and returns the challenge that was stored plus the mailed code.
func issueOTP(t *testing.T, ctrl *gomock.Controller) (IAuthService, *authMocks, *dao.OTPChallenge, string) {
	t.Helper()
	var saved dao.OTPChallenge
	svc, m := createAuthService(ctrl, func(m *authMocks) {
		m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil).AnyTimes()
		m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound).Times(1)
		m.otps.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c *dao.OTPChallenge) error {
			saved = *c
			return nil
		})
	})
	require.Nil(t, svc.RequestOTP(context.Background(), &request.OTPRequestReq{Email: anu.Email}))
	return svc, m, &saved, codeRegexp.FindString(m.mailer.Snapshot()[0].Body)
}

func TestVerifyOTP(t *testing.T) {
	ctx := context.Background()
	t.Run("correct code is single use and issues tokens", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueOTP(t, ctrl)
		m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), anu.Email).Return(nil)
		var sess *dao.RefreshSession
		m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s *dao.RefreshSession) error {
			sess = s
			return nil
		})
		m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil)
		expectUserResponse(m)
		resp, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{Email: anu.Email, Code: code})
		require.Nil(t, herr)
		require.Equal(t, 900, resp.ExpiresIn)
		require.Equal(t, "owner", resp.User.Role)
		require.NotEqual(t, resp.RefreshToken, sess.TokenHash, "refresh token is stored hashed")
		require.Equal(t, sha(resp.RefreshToken), sess.TokenHash)
	})
	t.Run("wrong code counts an attempt and is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, _ := issueOTP(t, ctrl)
		m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(ch, nil)
		m.otps.EXPECT().AddAttempt(gomock.Any(), anu.Email).Return(1, nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{Email: anu.Email, Code: "000000"})
		require.NotNil(t, herr)
		status, _ := herr.Get()
		require.Equal(t, 401, status)
	})
	t.Run("fifth wrong attempt burns the challenge", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, _ := issueOTP(t, ctrl)
		m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(ch, nil)
		m.otps.EXPECT().AddAttempt(gomock.Any(), anu.Email).Return(5, nil)
		m.otps.EXPECT().Delete(gomock.Any(), anu.Email).Return(nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{Email: anu.Email, Code: "000000"})
		require.NotNil(t, herr)
	})
	t.Run("expired challenge is deleted and rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueOTP(t, ctrl)
		m.clock.SetOffset(11 * time.Minute)
		m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), anu.Email).Return(nil)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{Email: anu.Email, Code: code})
		require.NotNil(t, herr)
	})
	t.Run("user without membership gets 403 after verification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc, m, ch, code := issueOTP(t, ctrl)
		m.otps.EXPECT().Get(gomock.Any(), anu.Email).Return(ch, nil)
		m.otps.EXPECT().Delete(gomock.Any(), anu.Email).Return(nil)
		m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(nil, apperrors.ErrRepoNotFound)
		_, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{Email: anu.Email, Code: code})
		require.NotNil(t, herr)
		status, _ := herr.Get()
		require.Equal(t, 403, status)
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

// mintForTest obtains a real access token by driving the OTP flow against the same mocks.
func mintForTest(t *testing.T, svc IAuthService, m *authMocks) string {
	t.Helper()
	ctx := context.Background()
	var ch dao.OTPChallenge
	m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil).AnyTimes()
	m.otps.EXPECT().Get(gomock.Any(), anu.Email).DoAndReturn(func(context.Context, string) (*dao.OTPChallenge, error) {
		if ch.Email == "" {
			return nil, apperrors.ErrRepoNotFound
		}
		c := ch
		return &c, nil
	}).AnyTimes()
	m.otps.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c *dao.OTPChallenge) error {
		ch = *c
		return nil
	}).AnyTimes()
	m.otps.EXPECT().Delete(gomock.Any(), anu.Email).Return(nil).AnyTimes()
	m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil).Times(1)
	m.memberships.EXPECT().MembersOfSpace(gomock.Any(), "space1").Return([]dao.Membership{*anuMember}, nil).AnyTimes()
	m.users.EXPECT().ByID(gomock.Any(), anu.ID).Return(anu, nil).Times(1)
	require.Nil(t, svc.RequestOTP(ctx, &request.OTPRequestReq{Email: anu.Email}))
	code := codeRegexp.FindString(m.mailer.Snapshot()[len(m.mailer.Snapshot())-1].Body)
	resp, herr := svc.VerifyOTP(ctx, &request.OTPVerifyReq{Email: anu.Email, Code: code})
	require.Nil(t, herr)
	return resp.AccessToken
}
