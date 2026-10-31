package mongorepo

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

type otpRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewOTPRepo(db drivers.IMongoDB, log logger.ILogger) IOTPRepo { return &otpRepo{db: db, log: log} }

func (r *otpRepo) coll() *mongo.Collection { return r.db.Database().Collection(constants.CollOTP) }

func (r *otpRepo) Save(ctx context.Context, c *dao.OTPChallenge) error {
	d := *c
	d.Email = strings.ToLower(c.Email)
	_, err := r.coll().ReplaceOne(ctx, byID(d.ChallengeID), d, options.Replace().SetUpsert(true))
	return err
}

func (r *otpRepo) Get(ctx context.Context, challengeID string) (*dao.OTPChallenge, error) {
	var d dao.OTPChallenge
	if err := r.coll().FindOne(ctx, byID(challengeID)).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	return &d, nil
}

func (r *otpRepo) LatestByEmail(ctx context.Context, email string) (*dao.OTPChallenge, error) {
	var d dao.OTPChallenge
	err := r.coll().FindOne(ctx, bson.D{{Key: "email", Value: strings.ToLower(email)}},
		options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&d)
	if err != nil {
		return nil, mapErr(err)
	}
	return &d, nil
}

func (r *otpRepo) AddAttempt(ctx context.Context, challengeID string) (int, error) {
	var d dao.OTPChallenge
	err := r.coll().FindOneAndUpdate(ctx, byID(challengeID),
		bson.D{{Key: "$inc", Value: bson.D{{Key: "attempts", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&d)
	if err != nil {
		return 0, mapErr(err)
	}
	return d.Attempts, nil
}

func (r *otpRepo) Delete(ctx context.Context, challengeID string) error {
	_, err := r.coll().DeleteOne(ctx, byID(challengeID))
	return err
}
