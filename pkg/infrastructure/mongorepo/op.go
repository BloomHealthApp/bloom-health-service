package mongorepo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/bubu-bloom/service/pkg/constants"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/drivers"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/logger"
)

type opRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

// NewOpRepo returns the MongoDB-backed IOpRepo.
func NewOpRepo(db drivers.IMongoDB, log logger.ILogger) IOpRepo { return &opRepo{db: db, log: log} }

type opDoc struct {
	ID        string     `bson:"_id"`
	SpaceID   string     `bson:"spaceId"`
	Status    string     `bson:"status"`
	Reason    string     `bson:"reason"`
	Resolved  string     `bson:"resolved"`
	EntityTyp string     `bson:"entityType"`
	Entity    *entityDoc `bson:"entity"`
	CreatedAt time.Time  `bson:"createdAt"`
}

func (r *opRepo) coll() *mongo.Collection { return r.db.Database().Collection(constants.CollOps) }

func (r *opRepo) Get(ctx context.Context, id string) (*dao.OpRecord, error) {
	var d opDoc
	if err := r.coll().FindOne(ctx, byID(id)).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	rec := &dao.OpRecord{OperationID: d.ID, SpaceID: d.SpaceID, Status: d.Status, Reason: d.Reason, Resolved: d.Resolved, CreatedAt: d.CreatedAt}
	if d.Entity != nil {
		e, err := d.Entity.model(d.EntityTyp)
		if err != nil {
			return nil, err
		}
		rec.Entity = e
	}
	return rec, nil
}

func (r *opRepo) Save(ctx context.Context, rec *dao.OpRecord) error {
	d := opDoc{ID: rec.OperationID, SpaceID: rec.SpaceID, Status: rec.Status, Reason: rec.Reason, Resolved: rec.Resolved, CreatedAt: rec.CreatedAt}
	if rec.Entity != nil {
		d.EntityTyp = rec.Entity.Type
		ed := toDoc(rec.Entity, rec.Entity.Seq)
		d.Entity = &ed
	}
	if _, err := r.coll().InsertOne(ctx, d); err != nil {
		if isDup(err) {
			return apperrors.ErrRepoConflict
		}
		return err
	}
	return nil
}
