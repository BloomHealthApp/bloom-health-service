package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

type chatMocks struct {
	sessions *mongorepo.MockIChatSessionRepo
	messages *mongorepo.MockIChatMessageRepo
}

func createChatService(ctrl *gomock.Controller, setup func(m *chatMocks)) IChatService {
	m := &chatMocks{sessions: mongorepo.NewMockIChatSessionRepo(ctrl), messages: mongorepo.NewMockIChatMessageRepo(ctrl)}
	if setup != nil {
		setup(m)
	}
	return NewChatService(logger.NewDiscard(), &config.Store{}, m.sessions, m.messages, clock.NewFake())
}

func chatSession(title string) *dao.ChatSession {
	return &dao.ChatSession{ID: "s1", UserID: "u1", SpaceID: "space1", Title: title, ModelID: "groq-gpt-oss-120b"}
}

func sendReq() *request.SendChatMessageReq {
	return &request.SendChatMessageReq{SessionID: "s1", Content: "hello"}
}

// A failed message insert must release the session's in-flight turn, or the user would be locked out
// until the worker's reaper runs.
func TestChatSendReleasesTurnWhenInsertFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	var turnID string
	svc := createChatService(ctrl, func(m *chatMocks) {
		m.sessions.EXPECT().Get(gomock.Any(), "u1", "s1").Return(chatSession("Sleep"), nil)
		m.messages.EXPECT().CountUserMessagesSince(gomock.Any(), "u1", gomock.Any()).Return(0, nil)
		m.sessions.EXPECT().BeginTurn(gomock.Any(), "u1", "s1", gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _, id string, _ any) (int, error) { turnID = id; return 4, nil })
		m.messages.EXPECT().Insert(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("write failed"))
		m.sessions.EXPECT().EndTurn(gomock.Any(), "s1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _, id string) error { require.Equal(t, turnID, id); return nil })
	})
	_, herr := svc.SendMessage(context.Background(), actorWithRole(userrole.Owner), sendReq())
	require.NotNil(t, herr)
	code, _ := herr.Get()
	require.Equal(t, http.StatusInternalServerError, code)
}

func TestChatSendInFlightIsConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createChatService(ctrl, func(m *chatMocks) {
		m.sessions.EXPECT().Get(gomock.Any(), "u1", "s1").Return(chatSession("Sleep"), nil)
		m.messages.EXPECT().CountUserMessagesSince(gomock.Any(), "u1", gomock.Any()).Return(0, nil)
		m.sessions.EXPECT().BeginTurn(gomock.Any(), "u1", "s1", gomock.Any(), gomock.Any()).Return(0, apperrors.ErrRepoConflict)
	})
	_, herr := svc.SendMessage(context.Background(), actorWithRole(userrole.Owner), sendReq())
	require.NotNil(t, herr)
	require.Equal(t, apperrors.CodeConflict, herr.GetCode())
}

func TestChatSendOverDailyCapTouchesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createChatService(ctrl, func(m *chatMocks) {
		m.sessions.EXPECT().Get(gomock.Any(), "u1", "s1").Return(chatSession("Sleep"), nil)
		m.messages.EXPECT().CountUserMessagesSince(gomock.Any(), "u1", gomock.Any()).Return(100, nil)
	})
	_, herr := svc.SendMessage(context.Background(), actorWithRole(userrole.Owner), sendReq())
	require.NotNil(t, herr)
	require.Equal(t, apperrors.CodeRateLimited, herr.GetCode())
	require.NotEmpty(t, herr.GetHeaders()["Retry-After"])
}

func TestChatFirstMessageNamesDefaultTitledSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createChatService(ctrl, func(m *chatMocks) {
		m.sessions.EXPECT().Get(gomock.Any(), "u1", "s1").Return(chatSession(constants.DefaultChatTitle), nil)
		m.messages.EXPECT().CountUserMessagesSince(gomock.Any(), "u1", gomock.Any()).Return(0, nil)
		m.sessions.EXPECT().BeginTurn(gomock.Any(), "u1", "s1", gomock.Any(), gomock.Any()).Return(0, nil)
		m.messages.EXPECT().Insert(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		m.sessions.EXPECT().SetTitle(gomock.Any(), "u1", "s1", "How do I build a habit of drinking water…").Return(nil)
	})
	req := &request.SendChatMessageReq{SessionID: "s1", Content: "How do I build a habit of drinking water\nevery single day?"}
	resp, herr := svc.SendMessage(context.Background(), actorWithRole(userrole.Viewer), req)
	require.Nil(t, herr)
	require.Equal(t, "pending", resp.AssistantMessage.Status)
	require.Equal(t, 1, resp.AssistantMessage.Idx)
}

// A chat created with a model that has since left the catalogue must not queue turns for it.
func TestChatSendUsesDefaultModelWhenChatModelWasRetired(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := createChatService(ctrl, func(m *chatMocks) {
		sess := chatSession("Sleep")
		sess.ModelID = "groq-llama-3.3-70b"
		m.sessions.EXPECT().Get(gomock.Any(), "u1", "s1").Return(sess, nil)
		m.messages.EXPECT().CountUserMessagesSince(gomock.Any(), "u1", gomock.Any()).Return(0, nil)
		m.sessions.EXPECT().BeginTurn(gomock.Any(), "u1", "s1", gomock.Any(), gomock.Any()).Return(2, nil)
		m.messages.EXPECT().Insert(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, msgs ...*dao.ChatMessage) error {
				for _, msg := range msgs {
					require.Equal(t, config.DefaultChatModels()[0].ID, msg.ModelID)
				}
				return nil
			})
	})
	resp, herr := svc.SendMessage(context.Background(), actorWithRole(userrole.Owner), sendReq())
	require.Nil(t, herr)
	require.Equal(t, "groq-gpt-oss-120b", resp.AssistantMessage.ModelID)
}
