package mongorepo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// IPushTokenRepo stores the Expo push tokens of users' devices.
type IPushTokenRepo interface {
	// Save stores t, moving the token to t.UserID if another account had it (one device, one signed-in user).
	Save(ctx context.Context, t *dao.PushToken) error
	// Delete removes token if it belongs to userID.
	Delete(ctx context.Context, token, userID string) error
	ForUser(ctx context.Context, userID string) ([]dao.PushToken, error)
	// DeleteTokens removes tokens that Expo reported as no longer registered.
	DeleteTokens(ctx context.Context, tokens []string) error
}

type pushTokenRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewPushTokenRepo(db drivers.IMongoDB, log logger.ILogger) IPushTokenRepo {
	return &pushTokenRepo{db: db, log: log}
}

func (r *pushTokenRepo) coll() *mongo.Collection {
	return r.db.Database().Collection(constants.CollPushTokens)
}

func (r *pushTokenRepo) Save(ctx context.Context, t *dao.PushToken) error {
	_, err := r.coll().ReplaceOne(ctx, byID(t.Token), t, options.Replace().SetUpsert(true))
	return err
}

func (r *pushTokenRepo) Delete(ctx context.Context, token, userID string) error {
	_, err := r.coll().DeleteOne(ctx, bson.D{{Key: "_id", Value: token}, {Key: "userId", Value: userID}})
	return err
}

func (r *pushTokenRepo) ForUser(ctx context.Context, userID string) ([]dao.PushToken, error) {
	cur, err := r.coll().Find(ctx, bson.D{{Key: "userId", Value: userID}})
	if err != nil {
		return nil, err
	}
	out := []dao.PushToken{}
	return out, cur.All(ctx, &out)
}

func (r *pushTokenRepo) DeleteTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	_, err := r.coll().DeleteMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: tokens}}}})
	return err
}
