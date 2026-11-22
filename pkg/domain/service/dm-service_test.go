package service

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

type dmMocks struct {
	users  *mongorepo.MockIUserRepo
	conns  *mongorepo.MockIConnectionRepo
	dms    *mongorepo.MockIDirectMessageRepo
	tokens *mongorepo.MockIPushTokenRepo
	push   *drivers.MockIPushSender
}

func createDMService(ctrl *gomock.Controller, setup func(m *dmMocks)) IDMService {
	m := &dmMocks{users: mongorepo.NewMockIUserRepo(ctrl), conns: mongorepo.NewMockIConnectionRepo(ctrl), dms: mongorepo.NewMockIDirectMessageRepo(ctrl),
		tokens: mongorepo.NewMockIPushTokenRepo(ctrl), push: drivers.NewMockIPushSender(ctrl)}
	setup(m)
	return NewDMService(logger.NewDiscard(), m.users, m.conns, m.dms, m.tokens, m.push, clock.NewFake())
}

func friendsWithU2(m *dmMocks) {
	m.conns.EXPECT().ByPair(gomock.Any(), "u1", "u2").Return(&dao.Connection{ID: "c1", PairKey: dao.PairKey("u1", "u2"), Status: dao.ConnectionAccepted}, nil)
	m.users.EXPECT().ByID(gomock.Any(), "u2").Return(&dao.User{ID: "u2", DisplayName: "Riya Sen"}, nil)
}

func TestSendStopsAtTheDailyCap(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createDMService(ctrl, func(m *dmMocks) {
		friendsWithU2(m)
		m.dms.EXPECT().ByID(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrRepoNotFound)
		m.dms.EXPECT().CountSentSince(gomock.Any(), "u1", gomock.Any()).Return(constants.DMDailyCap, nil)
	})
	_, _, herr := svc.Send(context.Background(), actorWithRole(userrole.Owner), &request.SendDMReq{FriendID: "u2", ID: uuid.NewString(), Text: "hi"})
	require.Equal(t, 429, status(herr))
}

func TestSendRefusesAnIdSomeoneElseUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	id := uuid.NewString()
	svc := createDMService(ctrl, func(m *dmMocks) {
		friendsWithU2(m)
		m.dms.EXPECT().ByID(gomock.Any(), id).Return(&dao.DirectMessage{ID: id, SenderID: "u9", PairKey: "other"}, nil)
	})
	_, _, herr := svc.Send(context.Background(), actorWithRole(userrole.Owner), &request.SendDMReq{FriendID: "u2", ID: id, Text: "hi"})
	require.Equal(t, 409, status(herr))
}

func TestPendingRequestCantMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createDMService(ctrl, func(m *dmMocks) {
		m.conns.EXPECT().ByPair(gomock.Any(), "u1", "u2").Return(&dao.Connection{Status: dao.ConnectionPending}, nil)
	})
	require.Equal(t, 404, status(svc.MarkRead(context.Background(), actorWithRole(userrole.Owner), "u2")))
}
