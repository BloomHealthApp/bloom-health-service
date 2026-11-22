package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/helpers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// IDMService handles text messages between friends and the push tokens used to tell them about new ones. Only
// accepted friends can message each other; for anyone else every call is the same 404, so it reveals nothing.
type IDMService interface {
	// Threads lists the caller's chats with friends, newest message first.
	Threads(ctx context.Context, actor *bo.Actor) (*response.DMThreadsResponse, apperrors.IHttpError)
	// Messages pages through a chat: older with before, newer (for polling) with after.
	Messages(ctx context.Context, actor *bo.Actor, req *request.ListDMsReq) (*response.DMPageResponse, apperrors.IHttpError)
	// Send saves a message and notifies the friend. created is false when the id was already sent (a retry).
	Send(ctx context.Context, actor *bo.Actor, req *request.SendDMReq) (msg *response.DMResponse, created bool, herr apperrors.IHttpError)
	// MarkRead records that the caller has read the chat with friendID up to now.
	MarkRead(ctx context.Context, actor *bo.Actor, friendID string) apperrors.IHttpError
	SavePushToken(ctx context.Context, actor *bo.Actor, req *request.PushTokenReq) apperrors.IHttpError
	DeletePushToken(ctx context.Context, actor *bo.Actor, req *request.PushTokenReq) apperrors.IHttpError
}

type dmService struct {
	log         logger.ILogger
	users       mongorepo.IUserRepo
	connections mongorepo.IConnectionRepo
	dms         mongorepo.IDirectMessageRepo
	push        drivers.IPushSender
	clock       clock.Clock
}

func NewDMService(log logger.ILogger, users mongorepo.IUserRepo, connections mongorepo.IConnectionRepo, dms mongorepo.IDirectMessageRepo,
	push drivers.IPushSender, clk clock.Clock) IDMService {
	return &dmService{log: log, users: users, connections: connections, dms: dms, push: push, clock: clk}
}

var errNoChat = apperrors.NotFoundError("chat not found")

// friend is friendID's account and the pair key when they are the caller's accepted friend, and errNoChat otherwise.
func (s *dmService) friend(ctx context.Context, me, friendID string) (*dao.User, string, apperrors.IHttpError) {
	if friendID == "" || friendID == me {
		return nil, "", errNoChat
	}
	c, err := s.connections.ByPair(ctx, me, friendID)
	if apperrors.IsNotFound(err) {
		return nil, "", errNoChat
	}
	if err != nil {
		return nil, "", internalErr(s.log, ctx, "dm: connection lookup failed", err)
	}
	if c.Status != dao.ConnectionAccepted {
		return nil, "", errNoChat
	}
	u, err := s.users.ByID(ctx, friendID)
	if apperrors.IsNotFound(err) {
		return nil, "", errNoChat
	}
	if err != nil {
		return nil, "", internalErr(s.log, ctx, "dm: user lookup failed", err)
	}
	if u.PendingDeletion() {
		return nil, "", errNoChat
	}
	return u, c.PairKey, nil
}

func dmResponse(m *dao.DirectMessage) response.DMResponse {
	return response.DMResponse{ID: m.ID, Seq: m.Seq, SenderID: m.SenderID, Text: m.Text, CreatedAt: m.CreatedAt}
}

func otherMember(t *dao.DMThread, me string) string {
	for _, m := range t.Members {
		if m != me {
			return m
		}
	}
	return ""
}

func unread(t *dao.DMThread, me string) bool {
	return t.Last != nil && t.Last.SenderID != me && t.Last.CreatedAt.After(t.ReadAt[me])
}

func (s *dmService) Threads(ctx context.Context, actor *bo.Actor) (*response.DMThreadsResponse, apperrors.IHttpError) {
	me := actor.User.ID
	conns, err := s.connections.ListForUser(ctx, me, constants.MaxDMThreadsListed)
	if err != nil {
		return nil, internalErr(s.log, ctx, "dm: connections failed", err)
	}
	// Only current friends: a chat whose deletion after an unfriend failed must not show.
	friends := map[string]bool{}
	for i := range conns {
		if conns[i].Status == dao.ConnectionAccepted {
			friends[conns[i].PairKey] = true
		}
	}
	threads, err := s.dms.ThreadsFor(ctx, me, constants.MaxDMThreadsListed)
	if err != nil {
		return nil, internalErr(s.log, ctx, "dm: threads failed", err)
	}
	out := &response.DMThreadsResponse{Items: []response.DMThreadResponse{}}
	for i := range threads {
		t := &threads[i]
		if !friends[t.ID] {
			continue
		}
		u, err := s.users.ByID(ctx, otherMember(t, me))
		if apperrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, internalErr(s.log, ctx, "dm: user lookup failed", err)
		}
		if u.PendingDeletion() {
			continue
		}
		item := response.DMThreadResponse{Friend: helpers.PublicProfileResponse(u, nil), Unread: unread(t, me)}
		if t.Last != nil {
			item.Last = &response.DMLastResponse{Text: t.Last.Text, Mine: t.Last.SenderID == me, CreatedAt: t.Last.CreatedAt}
		}
		if item.Unread {
			out.UnreadCount++
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func (s *dmService) Messages(ctx context.Context, actor *bo.Actor, req *request.ListDMsReq) (*response.DMPageResponse, apperrors.IHttpError) {
	friend, pairKey, herr := s.friend(ctx, actor.User.ID, req.FriendID)
	if herr != nil {
		return nil, herr
	}
	out := &response.DMPageResponse{Items: []response.DMResponse{}}
	t, err := s.dms.Thread(ctx, pairKey)
	if apperrors.IsNotFound(err) {
		return out, nil
	}
	if err != nil {
		return nil, internalErr(s.log, ctx, "dm: thread failed", err)
	}
	if at, ok := t.ReadAt[friend.ID]; ok {
		out.SeenAt = &at
	}
	msgs, err := s.dms.List(ctx, pairKey, req.Before, req.After, req.Limit+1)
	if err != nil {
		return nil, internalErr(s.log, ctx, "dm: list failed", err)
	}
	out.HasMore = len(msgs) > req.Limit
	if out.HasMore {
		msgs = msgs[:req.Limit]
	}
	if req.After <= 0 {
		// Fetched newest first; answer oldest first, with the cursor for the next older page.
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
		if out.HasMore {
			oldest := msgs[0].Seq
			out.NextBefore = &oldest
		}
	}
	for i := range msgs {
		out.Items = append(out.Items, dmResponse(&msgs[i]))
	}
	return out, nil
}

func (s *dmService) checkDailyCap(ctx context.Context, me string, now time.Time) apperrors.IHttpError {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	n, err := s.dms.CountSentSince(ctx, me, dayStart)
	if err != nil {
		return internalErr(s.log, ctx, "dm: daily count failed", err)
	}
	if n >= constants.DMDailyCap {
		retry := int(dayStart.Add(24*time.Hour).Sub(now).Seconds()) + 1
		return apperrors.TooManyRequestsError("daily message limit reached, try again tomorrow", strconv.Itoa(retry))
	}
	return nil
}

// sameSend returns an already-saved message with this id when it is the caller's resend to the same chat.
func (s *dmService) sameSend(ctx context.Context, id, me, pairKey string) (*dao.DirectMessage, apperrors.IHttpError) {
	m, err := s.dms.ByID(ctx, id)
	if apperrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, internalErr(s.log, ctx, "dm: lookup failed", err)
	}
	if m.SenderID != me || m.PairKey != pairKey {
		return nil, apperrors.ConflictError("message id already used", nil)
	}
	return m, nil
}

func (s *dmService) Send(ctx context.Context, actor *bo.Actor, req *request.SendDMReq) (*response.DMResponse, bool, apperrors.IHttpError) {
	me := actor.User.ID
	friend, pairKey, herr := s.friend(ctx, me, req.FriendID)
	if herr != nil {
		return nil, false, herr
	}
	if prev, herr := s.sameSend(ctx, req.ID, me, pairKey); herr != nil || prev != nil {
		if herr != nil {
			return nil, false, herr
		}
		r := dmResponse(prev)
		return &r, false, nil
	}
	now := s.clock.Now()
	if herr := s.checkDailyCap(ctx, me, now); herr != nil {
		return nil, false, herr
	}
	seq, err := s.dms.NextSeq(ctx, pairKey, []string{me, friend.ID})
	if err != nil {
		return nil, false, internalErr(s.log, ctx, "dm: seq failed", err)
	}
	m := &dao.DirectMessage{ID: req.ID, PairKey: pairKey, Seq: seq, SenderID: me, Text: req.Text, CreatedAt: now}
	if err := s.dms.Insert(ctx, m); err == apperrors.ErrRepoConflict {
		// The same id raced in from a parallel retry.
		prev, herr := s.sameSend(ctx, req.ID, me, pairKey)
		if herr != nil {
			return nil, false, herr
		}
		if prev != nil {
			r := dmResponse(prev)
			return &r, false, nil
		}
		return nil, false, apperrors.ConflictError("message id already used", nil)
	} else if err != nil {
		return nil, false, internalErr(s.log, ctx, "dm: insert failed", err)
	}
	if err := s.dms.SetLast(ctx, m); err != nil {
		return nil, false, internalErr(s.log, ctx, "dm: thread update failed", err)
	}
	// Sending means you've read everything before it.
	if err := s.dms.MarkRead(ctx, pairKey, me, now); err != nil {
		s.log.Warn(ctx, "dm: mark read failed", map[string]any{"err": err.Error()})
	}
	s.notify(ctx, actor.User, friend)
	r := dmResponse(m)
	return &r, true, nil
}

// notify pushes "<name> sent you a message" to the friend's devices. It never includes the message text, which would
// show on the lock screen, and a failure never fails the send.
func (s *dmService) notify(ctx context.Context, from, to *dao.User) {
	tokens := to.PushTokens
	if len(tokens) == 0 {
		return
	}
	name := "A friend"
	if f := strings.Fields(from.DisplayName); len(f) > 0 {
		name = f[0]
	}
	msgs := make([]drivers.PushMessage, 0, len(tokens))
	for _, t := range tokens {
		msgs = append(msgs, drivers.PushMessage{To: t.Token, Title: "Bloom", Body: name + " sent you a message",
			Data: map[string]string{"type": "dm", "from": from.ID}, ChannelID: "messages", Sound: "default"})
	}
	pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	res, err := s.push.Send(pctx, msgs)
	if err != nil {
		s.log.Warn(ctx, "dm: push failed", map[string]any{"err": err.Error()})
		return
	}
	if len(res.Unregistered) > 0 {
		if err := s.users.RemovePushTokens(ctx, res.Unregistered); err != nil {
			s.log.Warn(ctx, "dm: stale token cleanup failed", map[string]any{"err": err.Error()})
		}
	}
}

func (s *dmService) MarkRead(ctx context.Context, actor *bo.Actor, friendID string) apperrors.IHttpError {
	_, pairKey, herr := s.friend(ctx, actor.User.ID, friendID)
	if herr != nil {
		return herr
	}
	if err := s.dms.MarkRead(ctx, pairKey, actor.User.ID, s.clock.Now()); err != nil {
		return internalErr(s.log, ctx, "dm: mark read failed", err)
	}
	return nil
}

func (s *dmService) SavePushToken(ctx context.Context, actor *bo.Actor, req *request.PushTokenReq) apperrors.IHttpError {
	t := dao.PushToken{Token: req.Token, Platform: req.Platform, UpdatedAt: s.clock.Now()}
	if err := s.users.AddPushToken(ctx, actor.User.ID, t, constants.MaxPushTokensPerUser); err != nil {
		return internalErr(s.log, ctx, "push token save failed", err)
	}
	return nil
}

func (s *dmService) DeletePushToken(ctx context.Context, actor *bo.Actor, req *request.PushTokenReq) apperrors.IHttpError {
	if err := s.users.RemovePushToken(ctx, actor.User.ID, req.Token); err != nil {
		return internalErr(s.log, ctx, "push token delete failed", err)
	}
	return nil
}
