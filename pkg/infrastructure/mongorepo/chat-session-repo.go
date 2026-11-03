package mongorepo

import (
	"context"
	"errors"
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

// IChatSessionRepo persists chat sessions. Every lookup is scoped to the owning user, and
// soft-deleted sessions are reported as errors.ErrRepoNotFound.
type IChatSessionRepo interface {
	Create(ctx context.Context, s *dao.ChatSession) error
	Get(ctx context.Context, userID, id string) (*dao.ChatSession, error)
	// ListForUser returns non-deleted sessions, most recently updated first.
	ListForUser(ctx context.Context, userID string, limit int) ([]dao.ChatSession, error)
	// SoftDelete marks the session deleted and clears its title. The tombstone expires after constants.DeletedChatRetention.
	SoftDelete(ctx context.Context, userID, id string, at time.Time) error
	// BeginTurn marks turnID as the session's in-flight turn and reserves two message indexes,
	// returning the first. errors.ErrRepoConflict when another turn is already in flight.
	BeginTurn(ctx context.Context, userID, id, turnID string, at time.Time) (int, error)
	// EndTurn clears the in-flight turn only if it is still turnID.
	EndTurn(ctx context.Context, id, turnID string) error
	SetTitle(ctx context.Context, userID, id, title string) error
}

type chatSessionRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewChatSessionRepo(db drivers.IMongoDB, log logger.ILogger) IChatSessionRepo {
	return &chatSessionRepo{db: db, log: log}
}

func (r *chatSessionRepo) coll() *mongo.Collection {
	return r.db.Database().Collection(constants.CollChatSessions)
}

func liveSession(userID, id string) bson.D {
	return bson.D{{Key: "_id", Value: id}, {Key: "userId", Value: userID}, {Key: "deletedAt", Value: nil}}
}

func (r *chatSessionRepo) Create(ctx context.Context, s *dao.ChatSession) error {
	_, err := r.coll().InsertOne(ctx, s)
	if isDup(err) {
		return apperrors.ErrRepoConflict
	}
	return err
}

func (r *chatSessionRepo) Get(ctx context.Context, userID, id string) (*dao.ChatSession, error) {
	var s dao.ChatSession
	if err := r.coll().FindOne(ctx, liveSession(userID, id)).Decode(&s); err != nil {
		return nil, mapErr(err)
	}
	return &s, nil
}

func (r *chatSessionRepo) ListForUser(ctx context.Context, userID string, limit int) ([]dao.ChatSession, error) {
	cur, err := r.coll().Find(ctx, bson.D{{Key: "userId", Value: userID}, {Key: "deletedAt", Value: nil}},
		options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []dao.ChatSession{}
	return out, cur.All(ctx, &out)
}

func (r *chatSessionRepo) SoftDelete(ctx context.Context, userID, id string, at time.Time) error {
	res, err := r.coll().UpdateOne(ctx, liveSession(userID, id),
		bson.D{{Key: "$set", Value: bson.D{{Key: "deletedAt", Value: at}, {Key: "updatedAt", Value: at}, {Key: "title", Value: ""}}},
			{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return apperrors.ErrRepoNotFound
	}
	return nil
}

func (r *chatSessionRepo) BeginTurn(ctx context.Context, userID, id, turnID string, at time.Time) (int, error) {
	filter := append(liveSession(userID, id), bson.E{Key: "activeTurnId", Value: nil})
	var before dao.ChatSession
	err := r.coll().FindOneAndUpdate(ctx, filter,
		bson.D{{Key: "$set", Value: bson.D{{Key: "activeTurnId", Value: turnID}, {Key: "updatedAt", Value: at}}},
			{Key: "$inc", Value: bson.D{{Key: "nextIdx", Value: 2}, {Key: "version", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&before)
	if errors.Is(err, mongo.ErrNoDocuments) {
		if _, gerr := r.Get(ctx, userID, id); gerr != nil {
			return 0, gerr
		}
		return 0, apperrors.ErrRepoConflict
	}
	if err != nil {
		return 0, err
	}
	return before.NextIdx, nil
}

func (r *chatSessionRepo) EndTurn(ctx context.Context, id, turnID string) error {
	_, err := r.coll().UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "activeTurnId", Value: turnID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "activeTurnId", Value: nil}}}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}})
	return err
}

func (r *chatSessionRepo) SetTitle(ctx context.Context, userID, id, title string) error {
	res, err := r.coll().UpdateOne(ctx, liveSession(userID, id),
		bson.D{{Key: "$set", Value: bson.D{{Key: "title", Value: title}}}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return apperrors.ErrRepoNotFound
	}
	return nil
}
