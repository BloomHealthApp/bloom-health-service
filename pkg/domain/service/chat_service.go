package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/chatrole"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/chatstatus"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// IChatService is the AI chat API. It stores turns and enqueues pending assistant messages; the model
// is called by bloom-health-task-worker. Sessions belong to one user, whatever their role.
type IChatService interface {
	ListModels(ctx context.Context) *response.ChatModelsResponse
	CreateSession(ctx context.Context, actor *serviceBos.Actor, req *request.CreateChatSessionReq) (*response.ChatSessionResponse, apperrors.IHttpError)
	ListSessions(ctx context.Context, actor *serviceBos.Actor) (*response.ChatSessionsResponse, apperrors.IHttpError)
	GetSession(ctx context.Context, actor *serviceBos.Actor, id string) (*response.ChatSessionResponse, apperrors.IHttpError)
	DeleteSession(ctx context.Context, actor *serviceBos.Actor, id string) apperrors.IHttpError
	ListMessages(ctx context.Context, actor *serviceBos.Actor, req *request.ListChatMessagesReq) (*response.ChatMessagesResponse, apperrors.IHttpError)
	// SendMessage stores the user's message and a pending assistant message. 409 while a reply is in
	// flight, 429 past the daily cap.
	SendMessage(ctx context.Context, actor *serviceBos.Actor, req *request.SendChatMessageReq) (*response.SendChatMessageResponse, apperrors.IHttpError)
}

type chatService struct {
	log      logger.ILogger
	cfg      config.ChatConfig
	sessions mongorepo.IChatSessionRepo
	messages mongorepo.IChatMessageRepo
	clock    clock.Clock
}

func NewChatService(log logger.ILogger, cfg *config.Store, sessions mongorepo.IChatSessionRepo, messages mongorepo.IChatMessageRepo, clk clock.Clock) IChatService {
	return &chatService{log: log, cfg: cfg.Chat.WithDefaults(), sessions: sessions, messages: messages, clock: clk}
}

const autoTitleChars = 40

func (s *chatService) ListModels(_ context.Context) *response.ChatModelsResponse {
	out := &response.ChatModelsResponse{Items: make([]response.ChatModelResponse, 0, len(s.cfg.Models))}
	for _, m := range s.cfg.Models {
		out.Items = append(out.Items, response.ChatModelResponse{ID: m.ID, Label: m.Label, Vendor: m.Vendor, Default: m.Default})
	}
	return out
}

// resolveModel returns the catalogue id for an optional requested id, or fallback when none is given.
func (s *chatService) resolveModel(requested *string, fallback string) (string, apperrors.IHttpError) {
	if requested == nil {
		return fallback, nil
	}
	if _, ok := s.cfg.Model(*requested); !ok {
		return "", apperrors.BadRequestError("unknown modelId")
	}
	return *requested, nil
}

func (s *chatService) CreateSession(ctx context.Context, actor *serviceBos.Actor, req *request.CreateChatSessionReq) (*response.ChatSessionResponse, apperrors.IHttpError) {
	modelID, herr := s.resolveModel(req.ModelID, s.cfg.DefaultModel().ID)
	if herr != nil {
		return nil, herr
	}
	title := constants.DefaultChatTitle
	if req.Title != nil {
		title = *req.Title
	}
	now := s.clock.Now().UTC()
	sess := &dao.ChatSession{ID: uuid.NewString(), UserID: actor.User.ID, SpaceID: actor.SpaceID(), Title: title, ModelID: modelID,
		ContextScopes: []string{}, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return nil, internalErr(s.log, ctx, "chat session create failed", err)
	}
	r := chatSessionResponse(sess)
	return &r, nil
}

func (s *chatService) ListSessions(ctx context.Context, actor *serviceBos.Actor) (*response.ChatSessionsResponse, apperrors.IHttpError) {
	list, err := s.sessions.ListForUser(ctx, actor.User.ID, constants.MaxChatSessionsListed)
	if err != nil {
		return nil, internalErr(s.log, ctx, "chat session list failed", err)
	}
	out := &response.ChatSessionsResponse{Items: make([]response.ChatSessionResponse, 0, len(list))}
	for i := range list {
		out.Items = append(out.Items, chatSessionResponse(&list[i]))
	}
	return out, nil
}

func (s *chatService) session(ctx context.Context, actor *serviceBos.Actor, id string) (*dao.ChatSession, apperrors.IHttpError) {
	sess, err := s.sessions.Get(ctx, actor.User.ID, id)
	if apperrors.IsNotFound(err) {
		return nil, apperrors.NotFoundError("chat session not found")
	}
	if err != nil {
		return nil, internalErr(s.log, ctx, "chat session lookup failed", err)
	}
	return sess, nil
}

func (s *chatService) GetSession(ctx context.Context, actor *serviceBos.Actor, id string) (*response.ChatSessionResponse, apperrors.IHttpError) {
	sess, herr := s.session(ctx, actor, id)
	if herr != nil {
		return nil, herr
	}
	r := chatSessionResponse(sess)
	return &r, nil
}

func (s *chatService) DeleteSession(ctx context.Context, actor *serviceBos.Actor, id string) apperrors.IHttpError {
	err := s.sessions.SoftDelete(ctx, actor.User.ID, id, s.clock.Now().UTC())
	if apperrors.IsNotFound(err) {
		return apperrors.NotFoundError("chat session not found")
	}
	if err != nil {
		return internalErr(s.log, ctx, "chat session delete failed", err)
	}
	return nil
}

func (s *chatService) ListMessages(ctx context.Context, actor *serviceBos.Actor, req *request.ListChatMessagesReq) (*response.ChatMessagesResponse, apperrors.IHttpError) {
	if _, herr := s.session(ctx, actor, req.SessionID); herr != nil {
		return nil, herr
	}
	list, err := s.messages.ListForSession(ctx, req.SessionID, req.Before, req.Limit+1)
	if err != nil {
		return nil, internalErr(s.log, ctx, "chat message list failed", err)
	}
	out := &response.ChatMessagesResponse{Items: make([]response.ChatMessageResponse, 0, len(list))}
	if len(list) > req.Limit {
		list = list[:req.Limit]
		out.HasMore = true
		oldest := list[len(list)-1].Idx
		out.NextBefore = &oldest
	}
	for i := len(list) - 1; i >= 0; i-- {
		out.Items = append(out.Items, chatMessageResponse(&list[i]))
	}
	return out, nil
}

func (s *chatService) SendMessage(ctx context.Context, actor *serviceBos.Actor, req *request.SendChatMessageReq) (*response.SendChatMessageResponse, apperrors.IHttpError) {
	sess, herr := s.session(ctx, actor, req.SessionID)
	if herr != nil {
		return nil, herr
	}
	modelID, herr := s.resolveModel(req.ModelID, sess.ModelID)
	if herr != nil {
		return nil, herr
	}
	now := s.clock.Now().UTC()
	if herr := s.checkDailyCap(ctx, actor.User.ID, now); herr != nil {
		return nil, herr
	}

	turnID := uuid.NewString()
	idx, err := s.sessions.BeginTurn(ctx, actor.User.ID, sess.ID, turnID, now)
	switch {
	case apperrors.IsConflict(err):
		return nil, apperrors.ConflictError("a reply is still being written", map[string]any{"reason": "turn_in_flight"})
	case apperrors.IsNotFound(err):
		return nil, apperrors.NotFoundError("chat session not found")
	case err != nil:
		return nil, internalErr(s.log, ctx, "chat turn begin failed", err)
	}

	userMsg := &dao.ChatMessage{ID: uuid.NewString(), SessionID: sess.ID, UserID: actor.User.ID, Idx: idx, Role: chatrole.User,
		Content: req.Content, Status: chatstatus.Succeeded, ModelID: modelID, RunAt: now, CreatedAt: now, UpdatedAt: now}
	assistantMsg := &dao.ChatMessage{ID: turnID, SessionID: sess.ID, UserID: actor.User.ID, Idx: idx + 1, Role: chatrole.Assistant,
		Status: chatstatus.Pending, ModelID: modelID, RunAt: now, CreatedAt: now, UpdatedAt: now}
	if err := s.messages.Insert(ctx, userMsg, assistantMsg); err != nil {
		// No transactions: release the session so the user can retry instead of waiting for the worker's reaper.
		if rerr := s.sessions.EndTurn(ctx, sess.ID, turnID); rerr != nil {
			s.log.Error(ctx, "chat turn rollback failed", map[string]any{"sessionId": sess.ID, "err": rerr.Error()})
		}
		return nil, internalErr(s.log, ctx, "chat message insert failed", err)
	}
	if idx == 0 && sess.Title == constants.DefaultChatTitle {
		if err := s.sessions.SetTitle(ctx, actor.User.ID, sess.ID, autoTitle(req.Content)); err != nil {
			s.log.Warn(ctx, "chat auto-title failed", map[string]any{"sessionId": sess.ID, "err": err.Error()})
		}
	}
	return &response.SendChatMessageResponse{UserMessage: chatMessageResponse(userMsg), AssistantMessage: chatMessageResponse(assistantMsg)}, nil
}

// checkDailyCap refuses a new message once the user has sent DailyMessageCap messages this UTC day.
func (s *chatService) checkDailyCap(ctx context.Context, userID string, now time.Time) apperrors.IHttpError {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	n, err := s.messages.CountUserMessagesSince(ctx, userID, dayStart)
	if err != nil {
		return internalErr(s.log, ctx, "chat daily count failed", err)
	}
	if n >= s.cfg.DailyMessageCap {
		retry := int(dayStart.Add(24*time.Hour).Sub(now).Seconds()) + 1
		return apperrors.TooManyRequestsError("daily chat limit reached, try again tomorrow", strconv.Itoa(retry))
	}
	return nil
}

func autoTitle(content string) string {
	t := strings.Join(strings.Fields(content), " ")
	if r := []rune(t); len(r) > autoTitleChars {
		t = strings.TrimSpace(string(r[:autoTitleChars])) + "…"
	}
	return t
}

func chatSessionResponse(s *dao.ChatSession) response.ChatSessionResponse {
	scopes := s.ContextScopes
	if scopes == nil {
		scopes = []string{}
	}
	return response.ChatSessionResponse{ID: s.ID, Title: s.Title, ModelID: s.ModelID, ContextScopes: scopes,
		PendingMessageID: s.ActiveTurnID, CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339)}
}

func chatMessageResponse(m *dao.ChatMessage) response.ChatMessageResponse {
	var code *string
	if m.ErrorCode != "" {
		c := m.ErrorCode
		code = &c
	}
	return response.ChatMessageResponse{ID: m.ID, SessionID: m.SessionID, Idx: m.Idx, Role: m.Role, Content: m.Content,
		Status: chatstatus.Wire(m.Status), ModelID: m.ModelID, ErrorCode: code,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: m.UpdatedAt.UTC().Format(time.RFC3339)}
}
