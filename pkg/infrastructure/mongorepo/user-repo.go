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
	// Save upserts; returns errors.ErrRepoConflict when the email, phone or Google account belongs to another user.
	Save(ctx context.Context, u *dao.User) error
	// DueForDeletion returns up to limit accounts whose deletion grace period ended at or before now.
	DueForDeletion(ctx context.Context, now time.Time, limit int) ([]dao.User, error)
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
