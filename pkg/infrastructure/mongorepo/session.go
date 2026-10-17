package mongorepo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/bubu-bloom/service/pkg/constants"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/drivers"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/logger"
)

type sessionRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

// NewSessionRepo returns the MongoDB-backed ISessionRepo.
func NewSessionRepo(db drivers.IMongoDB, log logger.ILogger) ISessionRepo {
	return &sessionRepo{db: db, log: log}
}

func (r *sessionRepo) coll() *mongo.Collection {
	return r.db.Database().Collection(constants.CollSessions)
}

func (r *sessionRepo) Save(ctx context.Context, s *dao.RefreshSession) error {
	_, err := r.coll().InsertOne(ctx, s)
	if isDup(err) {
		return apperrors.ErrRepoConflict
	}
	return err
}

func (r *sessionRepo) ByHash(ctx context.Context, hash string) (*dao.RefreshSession, error) {
	var s dao.RefreshSession
	if err := r.coll().FindOne(ctx, bson.D{{Key: "tokenHash", Value: hash}}).Decode(&s); err != nil {
		return nil, mapErr(err)
	}
	return &s, nil
}

func (r *sessionRepo) MarkUsed(ctx context.Context, id string) (bool, error) {
	res, err := r.coll().UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "usedAt", Value: nil}, {Key: "revokedAt", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "usedAt", Value: time.Now().UTC()}}}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

func (r *sessionRepo) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := r.coll().UpdateMany(ctx,
		bson.D{{Key: "familyId", Value: familyID}, {Key: "revokedAt", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "revokedAt", Value: time.Now().UTC()}}}})
	return err
}
