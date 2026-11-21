package service

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/friendrelation"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

type connMocks struct {
	users *mongorepo.MockIUserRepo
	conns *mongorepo.MockIConnectionRepo
	stats *MockIStatsService
	clock *clock.Fake
}

func createConnectionService(ctrl *gomock.Controller, setup func(m *connMocks)) IConnectionService {
	m := &connMocks{users: mongorepo.NewMockIUserRepo(ctrl), conns: mongorepo.NewMockIConnectionRepo(ctrl),
		stats: NewMockIStatsService(ctrl), clock: clock.NewFake()}
	if setup != nil {
		setup(m)
	}
	return NewConnectionService(logger.NewDiscard(), &config.Store{PublicBaseURL: "https://bloom.test"}, m.users, m.conns, m.stats, m.clock)
}

const theirCode = "abcDEF123xyz"

func linkUser(id string) *dao.User {
	code := theirCode
	return &dao.User{ID: id, DisplayName: "Riya Sen", Email: "riya@example.com", ProfileLinkCode: &code}
}

func status(herr apperrors.IHttpError) int {
	code, _ := herr.Get()
	return code
}

func TestSendRequestAcceptsWhenTheyAlreadyAsked(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createConnectionService(ctrl, func(m *connMocks) {
		m.users.EXPECT().ByLinkCode(gomock.Any(), theirCode).Return(linkUser("u2"), nil)
		m.conns.EXPECT().ByPair(gomock.Any(), "u1", "u2").Return(&dao.Connection{ID: "c1", RequesterID: "u2", AddresseeID: "u1", Status: dao.ConnectionPending}, nil)
		m.conns.EXPECT().Accept(gomock.Any(), "c1", "u1", gomock.Any()).Return(nil)
		m.stats.EXPECT().ProfileStats(gomock.Any(), "u2").Return(&bo.ProfileStats{CurrentStreak: 3, LongestStreak: 9, Habits: 5}, nil)
	})
	resp, herr := svc.SendRequest(context.Background(), actorWithRole(userrole.Owner), &request.SendConnectionReq{Code: theirCode})
	require.Nil(t, herr)
	require.Equal(t, friendrelation.Friends, resp.Relation)
	require.Equal(t, 3, resp.Profile.Stats.CurrentStreak)
}

func TestSendRequestWaitsOutADecline(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createConnectionService(ctrl, func(m *connMocks) {
		until := m.clock.Now().Add(48 * time.Hour)
		m.users.EXPECT().ByLinkCode(gomock.Any(), theirCode).Return(linkUser("u2"), nil)
		m.conns.EXPECT().ByPair(gomock.Any(), "u1", "u2").Return(&dao.Connection{ID: "c1", RequesterID: "u1", AddresseeID: "u2",
			Status: dao.ConnectionDeclined, ExpiresAt: &until}, nil)
	})
	_, herr := svc.SendRequest(context.Background(), actorWithRole(userrole.Owner), &request.SendConnectionReq{Code: theirCode})
	require.NotNil(t, herr)
	require.Equal(t, 429, status(herr))
	require.NotEmpty(t, herr.GetHeaders()["Retry-After"])
}

func TestProfileByLinkHidesStatsUntilFriends(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createConnectionService(ctrl, func(m *connMocks) {
		m.users.EXPECT().ByLinkCode(gomock.Any(), theirCode).Return(linkUser("u2"), nil)
		m.conns.EXPECT().ByPair(gomock.Any(), "u1", "u2").Return(nil, apperrors.ErrRepoNotFound)
	})
	resp, herr := svc.ProfileByLink(context.Background(), actorWithRole(userrole.Owner), theirCode)
	require.Nil(t, herr)
	require.Equal(t, friendrelation.None, resp.Relation)
	require.Nil(t, resp.Profile.Stats, "no stats before the request is accepted")
}

func TestProfileIsOnlyForFriends(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createConnectionService(ctrl, func(m *connMocks) {
		m.conns.EXPECT().ByPair(gomock.Any(), "u1", "u2").Return(&dao.Connection{ID: "c1", RequesterID: "u1", AddresseeID: "u2", Status: dao.ConnectionPending}, nil)
	})
	_, herr := svc.Profile(context.Background(), actorWithRole(userrole.Owner), "u2")
	require.Equal(t, 404, status(herr))
}

func TestPendingDeletionLinksLookUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createConnectionService(ctrl, func(m *connMocks) {
		u := linkUser("u2")
		at := time.Now().Add(time.Hour)
		u.DeleteAfter = &at
		m.users.EXPECT().ByLinkCode(gomock.Any(), theirCode).Return(u, nil).Times(2)
		m.users.EXPECT().ByLinkCode(gomock.Any(), "AAAAAAAAAAAA").Return(nil, apperrors.ErrRepoNotFound)
	})
	_, deleted := svc.ProfileByLink(context.Background(), actorWithRole(userrole.Owner), theirCode)
	_, unknown := svc.ProfileByLink(context.Background(), actorWithRole(userrole.Owner), "AAAAAAAAAAAA")
	require.Equal(t, deleted, unknown)
	_, ok := svc.PublicCard(context.Background(), theirCode)
	require.False(t, ok)
}

func TestPublicCardShowsAFirstNameOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createConnectionService(ctrl, func(m *connMocks) {
		m.users.EXPECT().ByLinkCode(gomock.Any(), theirCode).Return(linkUser("u2"), nil)
	})
	card, ok := svc.PublicCard(context.Background(), theirCode)
	require.True(t, ok)
	require.Equal(t, &bo.PublicCard{FirstName: "Riya", Initial: "R"}, card)
}

func TestLinkCodeRetriesOnAClash(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createConnectionService(ctrl, func(m *connMocks) {
		m.users.EXPECT().ByID(gomock.Any(), "u1").Return(&dao.User{ID: "u1"}, nil).Times(3)
		gomock.InOrder(
			m.users.EXPECT().Save(gomock.Any(), gomock.Any()).Return(apperrors.ErrRepoConflict),
			m.users.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil),
		)
	})
	resp, herr := svc.ProfileLink(context.Background(), actorWithRole(userrole.Owner))
	require.Nil(t, herr)
	require.True(t, request.IsProfileLinkCode(resp.Code))
	require.Equal(t, "https://bloom.test/u/"+resp.Code, resp.URL)
}
