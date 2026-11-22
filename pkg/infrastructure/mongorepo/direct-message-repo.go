package mongorepo

import (
	"context"
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

// IDirectMessageRepo persists conversations between friends (dmThreads) and their messages (directMessages).
type IDirectMessageRepo interface {
	// NextSeq reserves the next message number in the pair's thread, creating the thread on first use.
	NextSeq(ctx context.Context, pairKey string, members []string) (int64, error)
	// Insert saves a message; ErrRepoConflict if its ID already exists.
	Insert(ctx context.Context, m *dao.DirectMessage) error
	ByID(ctx context.Context, id string) (*dao.DirectMessage, error)
	// SetLast records m as the thread's newest message unless a newer one is already recorded.
	SetLast(ctx context.Context, m *dao.DirectMessage) error
	// List returns up to limit messages: with afterSeq > 0, those after it, oldest first; otherwise those before
	// beforeSeq (all when 0), newest first.
	List(ctx context.Context, pairKey string, beforeSeq, afterSeq int64, limit int) ([]dao.DirectMessage, error)
	Thread(ctx context.Context, pairKey string) (*dao.DMThread, error)
	// ThreadsFor returns userID's threads that have a message, newest message first, at most limit.
	ThreadsFor(ctx context.Context, userID string, limit int) ([]dao.DMThread, error)
	// MarkRead records that userID has read the thread up to at. It does nothing for a thread that doesn't exist.
	MarkRead(ctx context.Context, pairKey, userID string, at time.Time) error
	// CountSentSince counts the messages senderID sent at or after since.
	CountSentSince(ctx context.Context, senderID string, since time.Time) (int, error)
	// DeletePair removes the pair's thread and every message in it.
	DeletePair(ctx context.Context, pairKey string) error
}

type directMessageRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewDirectMessageRepo(db drivers.IMongoDB, log logger.ILogger) IDirectMessageRepo {
	return &directMessageRepo{db: db, log: log}
}

func (r *directMessageRepo) threads() *mongo.Collection {
	return r.db.Database().Collection(constants.CollDMThreads)
}

func (r *directMessageRepo) msgs() *mongo.Collection {
	return r.db.Database().Collection(constants.CollDMs)
}

func (r *directMessageRepo) NextSeq(ctx context.Context, pairKey string, members []string) (int64, error) {
	var t dao.DMThread
	err := r.threads().FindOneAndUpdate(ctx, byID(pairKey),
		bson.D{{Key: "$inc", Value: bson.D{{Key: "nextSeq", Value: 1}}}, {Key: "$setOnInsert", Value: bson.D{{Key: "members", Value: members}}}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&t)
	if err != nil {
		return 0, err
	}
	return t.NextSeq, nil
}

func (r *directMessageRepo) Insert(ctx context.Context, m *dao.DirectMessage) error {
	_, err := r.msgs().InsertOne(ctx, m)
	if isDup(err) {
		return apperrors.ErrRepoConflict
	}
	return err
}

func (r *directMessageRepo) ByID(ctx context.Context, id string) (*dao.DirectMessage, error) {
	var m dao.DirectMessage
	if err := r.msgs().FindOne(ctx, byID(id)).Decode(&m); err != nil {
		return nil, mapErr(err)
	}
	return &m, nil
}

func (r *directMessageRepo) SetLast(ctx context.Context, m *dao.DirectMessage) error {
	last := dao.DMLast{Seq: m.Seq, SenderID: m.SenderID, Text: m.Text, CreatedAt: m.CreatedAt}
	_, err := r.threads().UpdateOne(ctx,
		bson.D{{Key: "_id", Value: m.PairKey}, {Key: "last.seq", Value: bson.D{{Key: "$not", Value: bson.D{{Key: "$gte", Value: m.Seq}}}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "last", Value: last}}}})
	return err
}

func (r *directMessageRepo) List(ctx context.Context, pairKey string, beforeSeq, afterSeq int64, limit int) ([]dao.DirectMessage, error) {
	filter := bson.D{{Key: "pairKey", Value: pairKey}}
	sort := -1
	switch {
	case afterSeq > 0:
		filter = append(filter, bson.E{Key: "seq", Value: bson.D{{Key: "$gt", Value: afterSeq}}})
		sort = 1
	case beforeSeq > 0:
		filter = append(filter, bson.E{Key: "seq", Value: bson.D{{Key: "$lt", Value: beforeSeq}}})
	}
	cur, err := r.msgs().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "seq", Value: sort}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []dao.DirectMessage{}
	return out, cur.All(ctx, &out)
}

func (r *directMessageRepo) Thread(ctx context.Context, pairKey string) (*dao.DMThread, error) {
	var t dao.DMThread
	if err := r.threads().FindOne(ctx, byID(pairKey)).Decode(&t); err != nil {
		return nil, mapErr(err)
	}
	return &t, nil
}

func (r *directMessageRepo) ThreadsFor(ctx context.Context, userID string, limit int) ([]dao.DMThread, error) {
	cur, err := r.threads().Find(ctx,
		bson.D{{Key: "members", Value: userID}, {Key: "last", Value: bson.D{{Key: "$exists", Value: true}}}},
		options.Find().SetSort(bson.D{{Key: "last.createdAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []dao.DMThread{}
	return out, cur.All(ctx, &out)
}

func (r *directMessageRepo) MarkRead(ctx context.Context, pairKey, userID string, at time.Time) error {
	_, err := r.threads().UpdateOne(ctx, bson.D{{Key: "_id", Value: pairKey}, {Key: "members", Value: userID}},
		bson.D{{Key: "$max", Value: bson.D{{Key: "readAt." + userID, Value: at}}}})
	return err
}

func (r *directMessageRepo) CountSentSince(ctx context.Context, senderID string, since time.Time) (int, error) {
	n, err := r.msgs().CountDocuments(ctx, bson.D{{Key: "senderId", Value: senderID}, {Key: "createdAt", Value: bson.D{{Key: "$gte", Value: since}}}})
	return int(n), err
}

func (r *directMessageRepo) DeletePair(ctx context.Context, pairKey string) error {
	if _, err := r.msgs().DeleteMany(ctx, bson.D{{Key: "pairKey", Value: pairKey}}); err != nil {
		return err
	}
	_, err := r.threads().DeleteOne(ctx, byID(pairKey))
	return err
}
