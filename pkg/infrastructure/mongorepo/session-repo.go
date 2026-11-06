package mongorepo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// ISessionRepo stores rotating refresh sessions.
type ISessionRepo interface {
	Save(ctx context.Context, s *dao.RefreshSession) error
	ByHash(ctx context.Context, hash string) (*dao.RefreshSession, error)
	// MarkUsed atomically sets UsedAt; returns false if it was already used or revoked.
	MarkUsed(ctx context.Context, id string) (bool, error)
	RevokeFamily(ctx context.Context, familyID string) error
	// RevokeAllForUser revokes every refresh session of the user, signing out all devices.
	RevokeAllForUser(ctx context.Context, userID string) error
}

type sessionRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

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

func (r *sessionRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.coll().UpdateMany(ctx,
		bson.D{{Key: "userId", Value: userID}, {Key: "revokedAt", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "revokedAt", Value: time.Now().UTC()}}}})
	return err
}
