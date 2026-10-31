package mongorepo

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

type entityRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

// NewEntityRepo returns the MongoDB-backed IEntityRepo (one collection per entity type).
func NewEntityRepo(db drivers.IMongoDB, log logger.ILogger) IEntityRepo {
	return &entityRepo{db: db, log: log}
}

// entityDoc is the persisted shape of an entity. _id is "<spaceId>/<entityId>" so ids are unique per space.
type entityDoc struct {
	ID        string     `bson:"_id"`
	EID       string     `bson:"eid"`
	SpaceID   string     `bson:"spaceId"`
	Version   int        `bson:"version"`
	Seq       int64      `bson:"seq"`
	CreatedAt time.Time  `bson:"createdAt"`
	UpdatedAt time.Time  `bson:"updatedAt"`
	DeletedAt *time.Time `bson:"deletedAt"`
	Data      bson.M     `bson:"data"`
}

func docID(space, id string) string { return space + "/" + id }

func toDoc(e *dao.Entity, seq int64) entityDoc {
	return entityDoc{ID: docID(e.SpaceID, e.ID), EID: e.ID, SpaceID: e.SpaceID, Version: e.Version, Seq: seq,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt, Data: bson.M(e.Data)}
}

// normalize converts BSON values (bson.M / bson.A / bson.D) into plain JSON-style Go values.
func normalize(in bson.M) (map[string]any, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	return out, json.Unmarshal(b, &out)
}

func (d entityDoc) model(typ string) (*dao.Entity, error) {
	data, err := normalize(d.Data)
	if err != nil {
		return nil, err
	}
	return &dao.Entity{ID: d.EID, Type: typ, SpaceID: d.SpaceID, Version: d.Version, Seq: d.Seq,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, DeletedAt: d.DeletedAt, Data: data}, nil
}

func (r *entityRepo) coll(typ string) (*mongo.Collection, error) {
	c, ok := EntityCollections[typ]
	if !ok {
		return nil, errors.New("unknown entity type " + typ)
	}
	return r.db.Database().Collection(c), nil
}

// nextSeq returns the next per-space monotonic sequence used by sync pull cursors.
func (r *entityRepo) nextSeq(ctx context.Context, spaceID string) (int64, error) {
	var out struct {
		Seq int64 `bson:"seq"`
	}
	err := r.db.Database().Collection(constants.CollCounters).FindOneAndUpdate(ctx,
		byID(spaceID),
		bson.D{{Key: "$inc", Value: bson.D{{Key: "seq", Value: 1}}}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&out)
	return out.Seq, err
}

func (r *entityRepo) Get(ctx context.Context, spaceID, typ, id string) (*dao.Entity, error) {
	c, err := r.coll(typ)
	if err != nil {
		return nil, err
	}
	var d entityDoc
	if err := c.FindOne(ctx, byID(docID(spaceID, id))).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	return d.model(typ)
}

func (r *entityRepo) List(ctx context.Context, spaceID, typ string, q EntityQuery) ([]dao.Entity, error) {
	c, err := r.coll(typ)
	if err != nil {
		return nil, err
	}
	filter := bson.D{{Key: "spaceId", Value: spaceID}}
	if !q.IncludeDeleted {
		filter = append(filter, bson.E{Key: "deletedAt", Value: nil})
	}
	if q.From != "" || q.To != "" {
		rng := bson.D{}
		if q.From != "" {
			rng = append(rng, bson.E{Key: "$gte", Value: q.From})
		}
		if q.To != "" {
			rng = append(rng, bson.E{Key: "$lte", Value: q.To})
		}
		filter = append(filter, bson.E{Key: "data.localDate", Value: rng})
	}
	cur, err := c.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}, {Key: "eid", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []entityDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return toEntities(docs, typ)
}

func toEntities(docs []entityDoc, typ string) ([]dao.Entity, error) {
	out := make([]dao.Entity, 0, len(docs))
	for _, d := range docs {
		e, err := d.model(typ)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, nil
}

func (r *entityRepo) Save(ctx context.Context, e *dao.Entity) error {
	c, err := r.coll(e.Type)
	if err != nil {
		return err
	}
	seq, err := r.nextSeq(ctx, e.SpaceID)
	if err != nil {
		return err
	}
	doc := toDoc(e, seq)
	if e.Version == 1 {
		if _, err := c.InsertOne(ctx, doc); err != nil {
			if isDup(err) {
				return apperrors.ErrRepoConflict
			}
			return err
		}
	} else {
		res, err := c.ReplaceOne(ctx, bson.D{{Key: "_id", Value: doc.ID}, {Key: "version", Value: e.Version - 1}}, doc)
		if err != nil {
			if isDup(err) {
				return apperrors.ErrRepoConflict
			}
			return err
		}
		if res.MatchedCount == 0 {
			return apperrors.ErrRepoConflict
		}
	}
	e.Seq = seq
	return nil
}

func (r *entityRepo) Changes(ctx context.Context, spaceID string, after int64, limit int) ([]dao.Entity, error) {
	var all []dao.Entity
	for _, typ := range entitytype.Values() {
		c, err := r.coll(typ)
		if err != nil {
			return nil, err
		}
		opts := options.Find().SetSort(bson.D{{Key: "seq", Value: 1}})
		if limit > 0 {
			opts.SetLimit(int64(limit))
		}
		cur, err := c.Find(ctx, bson.D{{Key: "spaceId", Value: spaceID}, {Key: "seq", Value: bson.D{{Key: "$gt", Value: after}}}}, opts)
		if err != nil {
			return nil, err
		}
		var docs []entityDoc
		if err := cur.All(ctx, &docs); err != nil {
			return nil, err
		}
		es, err := toEntities(docs, typ)
		if err != nil {
			return nil, err
		}
		all = append(all, es...)
	}
	// merge across collections (seq is unique per space)
	sort.Slice(all, func(i, j int) bool { return all[i].Seq < all[j].Seq })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
