package mongorepo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/bubu-bloom/service/pkg/constants"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/drivers"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/logger"
)

type membershipRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

// NewMembershipRepo returns the MongoDB-backed IMembershipRepo.
func NewMembershipRepo(db drivers.IMongoDB, log logger.ILogger) IMembershipRepo {
	return &membershipRepo{db: db, log: log}
}

func (r *membershipRepo) coll() *mongo.Collection {
	return r.db.Database().Collection(constants.CollMemberships)
}

func (r *membershipRepo) ForUser(ctx context.Context, userID string) (*dao.Membership, error) {
	var m dao.Membership
	if err := r.coll().FindOne(ctx, bson.D{{Key: "userId", Value: userID}}).Decode(&m); err != nil {
		return nil, mapErr(err)
	}
	return &m, nil
}

func (r *membershipRepo) MembersOfSpace(ctx context.Context, spaceID string) ([]dao.Membership, error) {
	cur, err := r.coll().Find(ctx, bson.D{{Key: "sharedSpaceId", Value: spaceID}})
	if err != nil {
		return nil, err
	}
	var out []dao.Membership
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *membershipRepo) Save(ctx context.Context, m *dao.Membership) error {
	_, err := r.coll().ReplaceOne(ctx, byID(m.ID), m, options.Replace().SetUpsert(true))
	if isDup(err) {
		return apperrors.ErrRepoConflict
	}
	return err
}
