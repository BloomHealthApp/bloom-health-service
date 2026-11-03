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
	// to their email, and their chat sessions and messages.
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
	return r.deleteAll(ctx, []deletion{
		{constants.CollChatMessages, byUser},
		{constants.CollChatSessions, byUser},
		{constants.CollSessions, byUser},
		{constants.CollOTP, bson.D{{Key: "email", Value: strings.ToLower(u.Email)}}},
		{constants.CollMemberships, byUser},
		{constants.CollUsers, byID(u.ID)},
	})
}
