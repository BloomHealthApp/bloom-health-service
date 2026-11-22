package mongorepo

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// IAccountRepo hard-deletes accounts (DELETE /v1/me). Every method is idempotent, so a failed erase can be retried.
type IAccountRepo interface {
	// EraseSpace deletes a space and everything stored in it: entities of every type, applied operations, its sync
	// counter and all its memberships.
	EraseSpace(ctx context.Context, spaceID string) error
	// EraseUser deletes the user and everything tied to them: memberships, refresh sessions, pending codes mailed
	// to their email, their chat sessions and messages, their friend requests and friendships, the conversations they are in
	// (both sides' messages). Push tokens go with the user document.
	EraseUser(ctx context.Context, user *dao.User) error
}

type accountRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewAccountRepo(db drivers.IMongoDB, log logger.ILogger) IAccountRepo {
	return &accountRepo{db: db, log: log}
}

type deletion struct {
	coll   string
	filter bson.D
}

func (r *accountRepo) deleteAll(ctx context.Context, ds []deletion) error {
	for _, d := range ds {
		if _, err := r.db.Database().Collection(d.coll).DeleteMany(ctx, d.filter); err != nil {
			return err
		}
	}
	return nil
}

func (r *accountRepo) EraseSpace(ctx context.Context, spaceID string) error {
	ds := make([]deletion, 0, len(EntityCollections)+4)
	for _, c := range EntityCollections {
		ds = append(ds, deletion{c, bson.D{{Key: "spaceId", Value: spaceID}}})
	}
	// The space document goes last so a retry after a partial failure still finds the space's rows.
	ds = append(ds,
		deletion{constants.CollOps, bson.D{{Key: "spaceId", Value: spaceID}}},
		deletion{constants.CollCounters, byID(spaceID)},
		deletion{constants.CollMemberships, bson.D{{Key: "sharedSpaceId", Value: spaceID}}},
		deletion{constants.CollSpaces, byID(spaceID)},
	)
	return r.deleteAll(ctx, ds)
}

func (r *accountRepo) EraseUser(ctx context.Context, u *dao.User) error {
	byUser := bson.D{{Key: "userId", Value: u.ID}}
	// Messages are found through the user's threads, so they go before the threads (a retried erase still finds them).
	keys := []string{}
	if err := r.db.Database().Collection(constants.CollDMThreads).Distinct(ctx, "_id", bson.D{{Key: "members", Value: u.ID}}).Decode(&keys); err != nil {
		return err
	}
	return r.deleteAll(ctx, []deletion{
		{constants.CollDMs, bson.D{{Key: "pairKey", Value: bson.D{{Key: "$in", Value: keys}}}}},
		{constants.CollDMThreads, bson.D{{Key: "members", Value: u.ID}}},
		{constants.CollConnections, involving(u.ID)},
		{constants.CollChatMessages, byUser},
		{constants.CollChatSessions, byUser},
		{constants.CollSessions, byUser},
		{constants.CollOTP, bson.D{{Key: "email", Value: strings.ToLower(u.Email)}}},
		{constants.CollMemberships, byUser},
		{constants.CollUsers, byID(u.ID)},
	})
}
