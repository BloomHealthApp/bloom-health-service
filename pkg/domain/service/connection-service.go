package service

import (
	"context"
	"crypto/rand"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/helpers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/friendrelation"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// IConnectionService handles profile links, friend requests and friendships. A request can only be made through
// someone's profile link, and other users are only ever returned as a public profile.
type IConnectionService interface {
	// ProfileLink returns the caller's shareable link, creating its code on first use.
	ProfileLink(ctx context.Context, actor *bo.Actor) (*response.ProfileLinkResponse, apperrors.IHttpError)
	// ResetProfileLink replaces the caller's code, so links shared before stop working.
	ResetProfileLink(ctx context.Context, actor *bo.Actor) (*response.ProfileLinkResponse, apperrors.IHttpError)
	// ProfileByLink shows the profile behind a link and how the caller relates to it.
	ProfileByLink(ctx context.Context, actor *bo.Actor, code string) (*response.ProfileByLinkResponse, apperrors.IHttpError)
	// SendRequest asks the link's owner to be friends; it accepts at once when they had already asked the caller.
	SendRequest(ctx context.Context, actor *bo.Actor, req *request.SendConnectionReq) (*response.ProfileByLinkResponse, apperrors.IHttpError)
	List(ctx context.Context, actor *bo.Actor) (*response.ConnectionsResponse, apperrors.IHttpError)
	Accept(ctx context.Context, actor *bo.Actor, id string) (*response.ConnectionResponse, apperrors.IHttpError)
	Decline(ctx context.Context, actor *bo.Actor, id string) apperrors.IHttpError
	// Remove unfriends, or cancels the caller's own pending request.
	Remove(ctx context.Context, actor *bo.Actor, id string) apperrors.IHttpError
	// Profile returns a friend's (or the caller's own) public profile with stats; 404 for anyone else.
	Profile(ctx context.Context, actor *bo.Actor, userID string) (*response.PublicProfileResponse, apperrors.IHttpError)
	// PublicCard is what the signed-out web page may show for a link; false for an unknown, reset or deleted link.
	PublicCard(ctx context.Context, code string) (*bo.PublicCard, bool)
}

type connectionService struct {
	log         logger.ILogger
	cfg         *config.Store
	users       mongorepo.IUserRepo
	connections mongorepo.IConnectionRepo
	stats       IStatsService
	clock       clock.Clock
}

func NewConnectionService(log logger.ILogger, cfg *config.Store, users mongorepo.IUserRepo, connections mongorepo.IConnectionRepo,
	stats IStatsService, clk clock.Clock) IConnectionService {
	return &connectionService{log: log, cfg: cfg, users: users, connections: connections, stats: stats, clock: clk}
}

const linkAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func newLinkCode() (string, error) {
	b := make([]byte, constants.ProfileLinkCodeLen)
	max := big.NewInt(int64(len(linkAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = linkAlphabet[n.Int64()]
	}
	return string(b), nil
}

var errNoProfile = apperrors.NotFoundError("profile not found")

func (s *connectionService) linkResponse(code string) *response.ProfileLinkResponse {
	return &response.ProfileLinkResponse{Code: code, URL: s.cfg.PublicBaseURL + "/u/" + code}
}

// setLinkCode gives the user a new code, retrying on the (very unlikely) clash with another user's code.
func (s *connectionService) setLinkCode(ctx context.Context, userID string) (string, apperrors.IHttpError) {
	for attempt := 0; attempt < 3; attempt++ {
		u, err := s.users.ByID(ctx, userID)
		if err != nil {
			return "", internalErr(s.log, ctx, "profile link: user lookup failed", err)
		}
		code, err := newLinkCode()
		if err != nil {
			return "", internalErr(s.log, ctx, "profile link: random failed", err)
		}
		u.ProfileLinkCode = &code
		u.UpdatedAt = s.clock.Now().UTC()
		err = s.users.Save(ctx, u)
		if err == nil {
			return code, nil
		}
		if !apperrors.IsConflict(err) {
			return "", internalErr(s.log, ctx, "profile link: save failed", err)
		}
	}
	return "", apperrors.InternalServerError("something went wrong")
}

func (s *connectionService) ProfileLink(ctx context.Context, actor *bo.Actor) (*response.ProfileLinkResponse, apperrors.IHttpError) {
	u, err := s.users.ByID(ctx, actor.User.ID)
	if err != nil {
		return nil, internalErr(s.log, ctx, "profile link: user lookup failed", err)
	}
	if u.ProfileLinkCode != nil {
		return s.linkResponse(*u.ProfileLinkCode), nil
	}
	code, herr := s.setLinkCode(ctx, actor.User.ID)
	if herr != nil {
		return nil, herr
	}
	return s.linkResponse(code), nil
}

func (s *connectionService) ResetProfileLink(ctx context.Context, actor *bo.Actor) (*response.ProfileLinkResponse, apperrors.IHttpError) {
	code, herr := s.setLinkCode(ctx, actor.User.ID)
	if herr != nil {
		return nil, herr
	}
	return s.linkResponse(code), nil
}

// byLink finds a live account by link code. Unknown, reset and deleted links all answer the same way.
func (s *connectionService) byLink(ctx context.Context, code string) (*dao.User, apperrors.IHttpError) {
	if !request.IsProfileLinkCode(code) {
		return nil, errNoProfile
	}
	u, err := s.users.ByLinkCode(ctx, code)
	if apperrors.IsNotFound(err) || (err == nil && u.PendingDeletion()) {
		return nil, errNoProfile
	}
	if err != nil {
		return nil, internalErr(s.log, ctx, "profile link lookup failed", err)
	}
	return u, nil
}

// pair returns the live connection between the two users, or nil; an expired decline marker counts as none.
func (s *connectionService) pair(ctx context.Context, a, b string) (*dao.Connection, error) {
	c, err := s.connections.ByPair(ctx, a, b)
	if apperrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *connectionService) coolingDown(c *dao.Connection) bool {
	return c.Status == dao.ConnectionDeclined && c.ExpiresAt != nil && s.clock.Now().Before(*c.ExpiresAt)
}

func relationOf(me string, c *dao.Connection) (string, *string) {
	if c == nil || c.Status == dao.ConnectionDeclined {
		return friendrelation.None, nil
	}
	id := c.ID
	switch {
	case c.Status == dao.ConnectionAccepted:
		return friendrelation.Friends, &id
	case c.RequesterID == me:
		return friendrelation.Outgoing, &id
	default:
		return friendrelation.Incoming, &id
	}
}

// profileFor renders target as the caller may see it: stats only for friends and for the caller's own profile.
func (s *connectionService) profileFor(ctx context.Context, target *dao.User, withStats bool) (response.PublicProfileResponse, apperrors.IHttpError) {
	if !withStats {
		return helpers.PublicProfileResponse(target, nil), nil
	}
	st, err := s.stats.ProfileStats(ctx, target.ID)
	if err != nil {
		return response.PublicProfileResponse{}, internalErr(s.log, ctx, "profile stats failed", err)
	}
	return helpers.PublicProfileResponse(target, st), nil
}

func (s *connectionService) byLinkResponse(ctx context.Context, me string, target *dao.User, c *dao.Connection) (*response.ProfileByLinkResponse, apperrors.IHttpError) {
	rel, id := relationOf(me, c)
	if target.ID == me {
		rel, id = friendrelation.Self, nil
	}
	p, herr := s.profileFor(ctx, target, rel == friendrelation.Friends || rel == friendrelation.Self)
	if herr != nil {
		return nil, herr
	}
	return &response.ProfileByLinkResponse{Profile: p, Relation: rel, ConnectionID: id}, nil
}

func (s *connectionService) ProfileByLink(ctx context.Context, actor *bo.Actor, code string) (*response.ProfileByLinkResponse, apperrors.IHttpError) {
	target, herr := s.byLink(ctx, code)
	if herr != nil {
		return nil, herr
	}
	if target.ID == actor.User.ID {
		return s.byLinkResponse(ctx, actor.User.ID, target, nil)
	}
	c, err := s.pair(ctx, actor.User.ID, target.ID)
	if err != nil {
		return nil, internalErr(s.log, ctx, "connection lookup failed", err)
	}
	return s.byLinkResponse(ctx, actor.User.ID, target, c)
}

func (s *connectionService) SendRequest(ctx context.Context, actor *bo.Actor, req *request.SendConnectionReq) (*response.ProfileByLinkResponse, apperrors.IHttpError) {
	me := actor.User.ID
	target, herr := s.byLink(ctx, req.Code)
	if herr != nil {
		return nil, herr
	}
	if target.ID == me {
		return nil, apperrors.BadRequestError("this is your own link")
	}
	now := s.clock.Now().UTC()
	c, err := s.pair(ctx, me, target.ID)
	if err != nil {
		return nil, internalErr(s.log, ctx, "connection lookup failed", err)
	}
	if c != nil {
		switch {
		case c.Status == dao.ConnectionAccepted:
			return nil, apperrors.ConflictError("you are already friends", nil)
		case c.Status == dao.ConnectionPending && c.RequesterID == me:
			return nil, apperrors.ConflictError("request already sent", nil)
		case c.Status == dao.ConnectionPending:
			// They had already asked: asking back makes it mutual.
			return s.accept(ctx, me, target, c, now)
		case s.coolingDown(c) && c.RequesterID == me:
			wait := int(c.ExpiresAt.Sub(now).Seconds()) + 1
			return nil, apperrors.TooManyRequestsError("you can ask again later", strconv.Itoa(wait))
		}
		// An expired decline, or one the caller made themself: clear it and start over.
		if err := s.connections.Delete(ctx, c.ID, me); err != nil && !apperrors.IsNotFound(err) {
			return nil, internalErr(s.log, ctx, "connection reset failed", err)
		}
	}
	nc := &dao.Connection{ID: uuid.NewString(), PairKey: dao.PairKey(me, target.ID), RequesterID: me, AddresseeID: target.ID,
		Status: dao.ConnectionPending, CreatedAt: now}
	if err := s.connections.Create(ctx, nc); apperrors.IsConflict(err) {
		return nil, apperrors.ConflictError("request already sent", nil)
	} else if err != nil {
		return nil, internalErr(s.log, ctx, "connection create failed", err)
	}
	return s.byLinkResponse(ctx, me, target, nc)
}

func (s *connectionService) accept(ctx context.Context, me string, other *dao.User, c *dao.Connection, now time.Time) (*response.ProfileByLinkResponse, apperrors.IHttpError) {
	err := s.connections.Accept(ctx, c.ID, me, now)
	if apperrors.IsNotFound(err) {
		return nil, apperrors.ConflictError("the request changed, try again", nil)
	}
	if err != nil {
		return nil, internalErr(s.log, ctx, "connection accept failed", err)
	}
	c.Status, c.RespondedAt = dao.ConnectionAccepted, &now
	return s.byLinkResponse(ctx, me, other, c)
}

// liveUser loads the other side of a connection; false when that account is gone or pending deletion.
func (s *connectionService) liveUser(ctx context.Context, id string) (*dao.User, bool, error) {
	u, err := s.users.ByID(ctx, id)
	if apperrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return u, !u.PendingDeletion(), nil
}

func (s *connectionService) List(ctx context.Context, actor *bo.Actor) (*response.ConnectionsResponse, apperrors.IHttpError) {
	me := actor.User.ID
	list, err := s.connections.ListForUser(ctx, me, 3*constants.MaxConnectionsListed)
	if err != nil {
		return nil, internalErr(s.log, ctx, "connection list failed", err)
	}
	out := &response.ConnectionsResponse{Friends: []response.ConnectionResponse{}, Incoming: []response.ConnectionResponse{}, Outgoing: []response.ConnectionResponse{}}
	for i := range list {
		c := &list[i]
		var bucket *[]response.ConnectionResponse
		switch {
		case c.Status == dao.ConnectionAccepted:
			bucket = &out.Friends
		case c.RequesterID == me:
			bucket = &out.Outgoing
		default:
			bucket = &out.Incoming
		}
		if len(*bucket) >= constants.MaxConnectionsListed {
			continue
		}
		other, ok, err := s.liveUser(ctx, c.Other(me))
		if err != nil {
			return nil, internalErr(s.log, ctx, "connection list: user lookup failed", err)
		}
		if ok {
			*bucket = append(*bucket, helpers.ConnectionResponse(c, other))
		}
	}
	return out, nil
}

func (s *connectionService) Accept(ctx context.Context, actor *bo.Actor, id string) (*response.ConnectionResponse, apperrors.IHttpError) {
	me := actor.User.ID
	c, err := s.connections.ByID(ctx, id)
	if apperrors.IsNotFound(err) || (err == nil && (c.AddresseeID != me || c.Status != dao.ConnectionPending)) {
		return nil, apperrors.NotFoundError("request not found")
	}
	if err != nil {
		return nil, internalErr(s.log, ctx, "connection lookup failed", err)
	}
	other, ok, err := s.liveUser(ctx, c.RequesterID)
	if err != nil {
		return nil, internalErr(s.log, ctx, "connection accept: user lookup failed", err)
	}
	if !ok {
		return nil, apperrors.NotFoundError("request not found")
	}
	now := s.clock.Now().UTC()
	if err := s.connections.Accept(ctx, id, me, now); apperrors.IsNotFound(err) {
		return nil, apperrors.NotFoundError("request not found")
	} else if err != nil {
		return nil, internalErr(s.log, ctx, "connection accept failed", err)
	}
	c.Status, c.RespondedAt = dao.ConnectionAccepted, &now
	r := helpers.ConnectionResponse(c, other)
	return &r, nil
}

func (s *connectionService) Decline(ctx context.Context, actor *bo.Actor, id string) apperrors.IHttpError {
	now := s.clock.Now().UTC()
	err := s.connections.Decline(ctx, id, actor.User.ID, now, now.Add(constants.DeclinedRequestCooldown))
	if apperrors.IsNotFound(err) {
		return apperrors.NotFoundError("request not found")
	}
	if err != nil {
		return internalErr(s.log, ctx, "connection decline failed", err)
	}
	return nil
}

func (s *connectionService) Remove(ctx context.Context, actor *bo.Actor, id string) apperrors.IHttpError {
	me := actor.User.ID
	c, err := s.connections.ByID(ctx, id)
	notFound := apperrors.NotFoundError("connection not found")
	if apperrors.IsNotFound(err) {
		return notFound
	}
	if err != nil {
		return internalErr(s.log, ctx, "connection lookup failed", err)
	}
	// Friends can unfriend from either side; a pending request can only be withdrawn by the one who sent it
	// (the other side declines instead), and decline markers are never removable here.
	allowed := c.Status == dao.ConnectionAccepted && c.Involves(me) || c.Status == dao.ConnectionPending && c.RequesterID == me
	if !allowed {
		return notFound
	}
	if err := s.connections.Delete(ctx, id, me); apperrors.IsNotFound(err) {
		return notFound
	} else if err != nil {
		return internalErr(s.log, ctx, "connection delete failed", err)
	}
	return nil
}

func (s *connectionService) Profile(ctx context.Context, actor *bo.Actor, userID string) (*response.PublicProfileResponse, apperrors.IHttpError) {
	me := actor.User.ID
	if userID != me {
		c, err := s.pair(ctx, me, userID)
		if err != nil {
			return nil, internalErr(s.log, ctx, "connection lookup failed", err)
		}
		if c == nil || c.Status != dao.ConnectionAccepted {
			return nil, errNoProfile
		}
	}
	u, ok, err := s.liveUser(ctx, userID)
	if err != nil {
		return nil, internalErr(s.log, ctx, "profile: user lookup failed", err)
	}
	if !ok {
		return nil, errNoProfile
	}
	p, herr := s.profileFor(ctx, u, true)
	if herr != nil {
		return nil, herr
	}
	return &p, nil
}

func (s *connectionService) PublicCard(ctx context.Context, code string) (*bo.PublicCard, bool) {
	u, herr := s.byLink(ctx, code)
	if herr != nil {
		return nil, false
	}
	first := strings.Fields(u.DisplayName)
	name := "Someone"
	if len(first) > 0 {
		name = first[0]
	}
	r, _ := utf8.DecodeRuneInString(name)
	return &bo.PublicCard{FirstName: name, Initial: string(unicode.ToUpper(r))}, true
}
