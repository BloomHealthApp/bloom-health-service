package mongorepo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

type spaceRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

// NewSpaceRepo returns the MongoDB-backed ISpaceRepo.
func NewSpaceRepo(db drivers.IMongoDB, log logger.ILogger) ISpaceRepo {
	return &spaceRepo{db: db, log: log}
}

func (r *spaceRepo) coll() *mongo.Collection { return r.db.Database().Collection(constants.CollSpaces) }

func (r *spaceRepo) ByID(ctx context.Context, id string) (*dao.Space, error) {
	var s dao.Space
	if err := r.coll().FindOne(ctx, byID(id)).Decode(&s); err != nil {
		return nil, mapErr(err)
	}
	return &s, nil
}

func (r *spaceRepo) Save(ctx context.Context, s *dao.Space) error {
	_, err := r.coll().ReplaceOne(ctx, byID(s.ID), s, options.Replace().SetUpsert(true))
	return err
}
