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
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/chatrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// IChatMessageRepo persists chat messages.
type IChatMessageRepo interface {
	Insert(ctx context.Context, msgs ...*dao.ChatMessage) error
	// ListForSession returns up to limit messages with Idx < beforeIdx (all when beforeIdx <= 0),
	// newest first.
	ListForSession(ctx context.Context, sessionID string, beforeIdx, limit int) ([]dao.ChatMessage, error)
	// CountUserMessagesSince counts the user-role messages userID sent at or after since.
	CountUserMessagesSince(ctx context.Context, userID string, since time.Time) (int, error)
	// DeleteForSession hard-deletes every message of a session.
	DeleteForSession(ctx context.Context, sessionID string) error
}

type chatMessageRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewChatMessageRepo(db drivers.IMongoDB, log logger.ILogger) IChatMessageRepo {
	return &chatMessageRepo{db: db, log: log}
}

func (r *chatMessageRepo) coll() *mongo.Collection {
	return r.db.Database().Collection(constants.CollChatMessages)
}

func (r *chatMessageRepo) Insert(ctx context.Context, msgs ...*dao.ChatMessage) error {
	docs := make([]any, len(msgs))
	for i, m := range msgs {
		docs[i] = m
	}
	_, err := r.coll().InsertMany(ctx, docs, options.InsertMany().SetOrdered(true))
	if isDup(err) {
		return apperrors.ErrRepoConflict
	}
	return err
}

func (r *chatMessageRepo) ListForSession(ctx context.Context, sessionID string, beforeIdx, limit int) ([]dao.ChatMessage, error) {
	filter := bson.D{{Key: "sessionId", Value: sessionID}}
	if beforeIdx > 0 {
		filter = append(filter, bson.E{Key: "idx", Value: bson.D{{Key: "$lt", Value: beforeIdx}}})
	}
	cur, err := r.coll().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "idx", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []dao.ChatMessage{}
	return out, cur.All(ctx, &out)
}

func (r *chatMessageRepo) CountUserMessagesSince(ctx context.Context, userID string, since time.Time) (int, error) {
	n, err := r.coll().CountDocuments(ctx, bson.D{{Key: "userId", Value: userID}, {Key: "role", Value: chatrole.User},
		{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: since}}}})
	return int(n), err
}

func (r *chatMessageRepo) DeleteForSession(ctx context.Context, sessionID string) error {
	_, err := r.coll().DeleteMany(ctx, bson.D{{Key: "sessionId", Value: sessionID}})
	return err
}
