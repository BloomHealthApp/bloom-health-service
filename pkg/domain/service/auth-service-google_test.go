package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

var anuGoogle = &drivers.GoogleIdentity{Subject: "g-anu", Email: anu.Email, Name: "Anu G"}

// expectIssue covers the session and membership lookups issue performs for anu.
func expectIssue(m *authMocks) {
	m.sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil)
	expectUserResponse(m)
}

func linked(u *dao.User, sub string) *dao.User {
	c := *u
	c.GoogleSub = &sub
	return &c
}

func TestSignInWithGoogle(t *testing.T) {
	ctx := context.Background()
	req := &request.GoogleSignInReq{IDToken: "tok"}

	t.Run("a rejected token is a 401 and touches no account", func(t *testing.T) {
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(nil, errors.New("bad signature"))
		})
		_, herr := svc.SignInWithGoogle(ctx, req)
		requireStatus(t, herr, 401, "unauthorized")
	})
	t.Run("not configured is a 404", func(t *testing.T) {
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(nil, drivers.ErrGoogleSignInOff)
		})
		_, herr := svc.SignInWithGoogle(ctx, req)
		requireStatus(t, herr, 404, "not_found")
	})
	t.Run("an already linked account signs in", func(t *testing.T) {
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(anuGoogle, nil)
			m.users.EXPECT().ByGoogleSub(gomock.Any(), "g-anu").Return(linked(anu, "g-anu"), nil)
			expectIssue(m)
		})
		resp, herr := svc.SignInWithGoogle(ctx, req)
		require.Nil(t, herr)
		require.False(t, resp.IsNewUser)
		require.NotEmpty(t, resp.AccessToken)
	})
	t.Run("an email-code account with the same email is linked", func(t *testing.T) {
		var saved *dao.User
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(anuGoogle, nil)
			m.users.EXPECT().ByGoogleSub(gomock.Any(), "g-anu").Return(nil, apperrors.ErrRepoNotFound)
			m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(anu, nil)
			m.users.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u *dao.User) error {
				saved = u
				return nil
			})
			expectIssue(m)
		})
		resp, herr := svc.SignInWithGoogle(ctx, req)
		require.Nil(t, herr)
		require.False(t, resp.IsNewUser)
		require.Equal(t, anu.ID, saved.ID)
		require.Equal(t, "g-anu", *saved.GoogleSub)
		require.Equal(t, "Anu", saved.DisplayName, "linking keeps the existing profile")
	})
	t.Run("an email linked to a different Google account is a 409", func(t *testing.T) {
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(anuGoogle, nil)
			m.users.EXPECT().ByGoogleSub(gomock.Any(), "g-anu").Return(nil, apperrors.ErrRepoNotFound)
			m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(linked(anu, "g-other"), nil)
		})
		_, herr := svc.SignInWithGoogle(ctx, req)
		requireStatus(t, herr, 409, "already_registered")
	})
	t.Run("an unknown email creates an account with no phone", func(t *testing.T) {
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(anuGoogle, nil)
			m.users.EXPECT().ByGoogleSub(gomock.Any(), "g-anu").Return(nil, apperrors.ErrRepoNotFound)
			m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound)
			m.provision.EXPECT().EnsureAccount(gomock.Any(), "Anu G", "", "", "", anu.Email, gomock.Any()).Return(anu, nil)
			m.users.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			expectIssue(m)
		})
		resp, herr := svc.SignInWithGoogle(ctx, req)
		require.Nil(t, herr)
		require.True(t, resp.IsNewUser)
	})
	t.Run("a first sign-in that loses a race signs in to the winner's account", func(t *testing.T) {
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(anuGoogle, nil)
			gomock.InOrder(
				m.users.EXPECT().ByGoogleSub(gomock.Any(), "g-anu").Return(nil, apperrors.ErrRepoNotFound),
				m.users.EXPECT().ByGoogleSub(gomock.Any(), "g-anu").Return(linked(anu, "g-anu"), nil),
			)
			m.users.EXPECT().ByEmail(gomock.Any(), anu.Email).Return(nil, apperrors.ErrRepoNotFound)
			m.provision.EXPECT().EnsureAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, apperrors.ErrRepoConflict)
			expectIssue(m)
		})
		resp, herr := svc.SignInWithGoogle(ctx, req)
		require.Nil(t, herr)
		require.False(t, resp.IsNewUser)
	})
	t.Run("an account past its deletion date is erased and refused", func(t *testing.T) {
		past := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		gone := linked(anu, "g-anu")
		gone.DeleteAfter = &past
		svc, _ := createAuthService(gomock.NewController(t), func(m *authMocks) {
			m.google.EXPECT().Verify(gomock.Any(), "tok").Return(anuGoogle, nil)
			m.users.EXPECT().ByGoogleSub(gomock.Any(), "g-anu").Return(gone, nil)
			m.memberships.EXPECT().ForUser(gomock.Any(), anu.ID).Return(anuMember, nil)
			m.accounts.EXPECT().EraseSpace(gomock.Any(), anuMember.SpaceID).Return(nil)
			m.accounts.EXPECT().EraseUser(gomock.Any(), gone).Return(nil)
		})
		_, herr := svc.SignInWithGoogle(ctx, req)
		requireStatus(t, herr, 401, "unauthorized")
	})
}
