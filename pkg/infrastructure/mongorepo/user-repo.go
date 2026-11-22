package mongorepo

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// IUserRepo persists users. Lookups return errors.ErrRepoNotFound when absent.
type IUserRepo interface {
	ByEmail(ctx context.Context, email string) (*dao.User, error)
	// ByPhone looks up an E.164 number (legacy users have none).
	ByPhone(ctx context.Context, phone string) (*dao.User, error)
	// ByGoogleSub looks up the user linked to a Google account id.
	ByGoogleSub(ctx context.Context, sub string) (*dao.User, error)
	ByID(ctx context.Context, id string) (*dao.User, error)
	// ByLinkCode looks up the user whose profile link code is code.
	ByLinkCode(ctx context.Context, code string) (*dao.User, error)
	// Save upserts; returns errors.ErrRepoConflict when the email, phone, Google account or profile link code belongs to
	// another user.
	Save(ctx context.Context, u *dao.User) error
	// DueForDeletion returns up to limit accounts whose deletion grace period ended at or before now.
	DueForDeletion(ctx context.Context, now time.Time, limit int) ([]dao.User, error)
	// AddPushToken stores a device's push token on userID, taking it off any other user (one device, one signed-in
	// account) and keeping only the newest max tokens. These update the field in place, so they don't race with Save.
	AddPushToken(ctx context.Context, userID string, t dao.PushToken, max int) error
	// RemovePushToken removes token from userID (sign-out).
	RemovePushToken(ctx context.Context, userID, token string) error
	// RemovePushTokens removes tokens from whoever has them (Expo says they're no longer registered).
	RemovePushTokens(ctx context.Context, tokens []string) error
}

type userRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewUserRepo(db drivers.IMongoDB, log logger.ILogger) IUserRepo {
	return &userRepo{db: db, log: log}
}

func (r *userRepo) coll() *mongo.Collection { return r.db.Database().Collection(constants.CollUsers) }

func (r *userRepo) find(ctx context.Context, filter bson.D) (*dao.User, error) {
	var u dao.User
	if err := r.coll().FindOne(ctx, filter).Decode(&u); err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (r *userRepo) ByEmail(ctx context.Context, email string) (*dao.User, error) {
	return r.find(ctx, bson.D{{Key: "emailLower", Value: strings.ToLower(email)}})
}

func (r *userRepo) ByPhone(ctx context.Context, phone string) (*dao.User, error) {
	return r.find(ctx, bson.D{{Key: "phone", Value: phone}})
}

func (r *userRepo) ByGoogleSub(ctx context.Context, sub string) (*dao.User, error) {
	return r.find(ctx, bson.D{{Key: "googleSub", Value: sub}})
}

func (r *userRepo) ByID(ctx context.Context, id string) (*dao.User, error) {
	return r.find(ctx, byID(id))
}

func (r *userRepo) ByLinkCode(ctx context.Context, code string) (*dao.User, error) {
	return r.find(ctx, bson.D{{Key: "profileLinkCode", Value: code}})
}

func (r *userRepo) Save(ctx context.Context, u *dao.User) error {
	d := *u
	d.EmailLower = strings.ToLower(u.Email)
	_, err := r.coll().ReplaceOne(ctx, byID(u.ID), d, options.Replace().SetUpsert(true))
	if isDup(err) {
		return apperrors.ErrRepoConflict
	}
	return err
}

func (r *userRepo) DueForDeletion(ctx context.Context, now time.Time, limit int) ([]dao.User, error) {
	cur, err := r.coll().Find(ctx, bson.D{{Key: "deleteAfter", Value: bson.D{{Key: "$lte", Value: now}}}},
		options.Find().SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	var out []dao.User
	return out, cur.All(ctx, &out)
}

func pullTokens(tokens ...string) bson.D {
	return bson.D{{Key: "$pull", Value: bson.D{{Key: "pushTokens", Value: bson.D{{Key: "token", Value: bson.D{{Key: "$in", Value: tokens}}}}}}}}
}

func (r *userRepo) AddPushToken(ctx context.Context, userID string, t dao.PushToken, max int) error {
	if _, err := r.coll().UpdateMany(ctx, bson.D{{Key: "pushTokens.token", Value: t.Token}}, pullTokens(t.Token)); err != nil {
		return err
	}
	_, err := r.coll().UpdateOne(ctx, byID(userID), bson.D{{Key: "$push", Value: bson.D{{Key: "pushTokens",
		Value: bson.D{{Key: "$each", Value: bson.A{t}}, {Key: "$slice", Value: -max}}}}}})
	return err
}

func (r *userRepo) RemovePushToken(ctx context.Context, userID, token string) error {
	_, err := r.coll().UpdateOne(ctx, byID(userID), pullTokens(token))
	return err
}

func (r *userRepo) RemovePushTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	_, err := r.coll().UpdateMany(ctx, bson.D{{Key: "pushTokens.token", Value: bson.D{{Key: "$in", Value: tokens}}}}, pullTokens(tokens...))
	return err
}
