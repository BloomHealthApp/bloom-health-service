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
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

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
