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

// IConnectionRepo persists friend requests and friendships. Lookups return errors.ErrRepoNotFound when absent.
type IConnectionRepo interface {
	// Create inserts c; errors.ErrRepoConflict when the pair already has a connection of any status.
	Create(ctx context.Context, c *dao.Connection) error
	ByID(ctx context.Context, id string) (*dao.Connection, error)
	// ByPair returns the connection between two users, in either direction, whatever its status.
	ByPair(ctx context.Context, a, b string) (*dao.Connection, error)
	// ListForUser returns the pending and accepted connections involving userID, newest first, at most limit.
	ListForUser(ctx context.Context, userID string, limit int) ([]dao.Connection, error)
	// Accept marks the pending request id addressed to addresseeID as accepted.
	Accept(ctx context.Context, id, addresseeID string, at time.Time) error
	// Decline turns the pending request id addressed to addresseeID into a cooldown marker that expires at until.
	Decline(ctx context.Context, id, addresseeID string, at, until time.Time) error
	// Delete removes connection id if it involves userID.
	Delete(ctx context.Context, id, userID string) error
}

type connectionRepo struct {
	db  drivers.IMongoDB
	log logger.ILogger
}

func NewConnectionRepo(db drivers.IMongoDB, log logger.ILogger) IConnectionRepo {
	return &connectionRepo{db: db, log: log}
}

func (r *connectionRepo) coll() *mongo.Collection {
	return r.db.Database().Collection(constants.CollConnections)
}

func involving(userID string) bson.D {
	return bson.D{{Key: "$or", Value: bson.A{bson.D{{Key: "requesterId", Value: userID}}, bson.D{{Key: "addresseeId", Value: userID}}}}}
}

func (r *connectionRepo) Create(ctx context.Context, c *dao.Connection) error {
	_, err := r.coll().InsertOne(ctx, c)
	if isDup(err) {
		return apperrors.ErrRepoConflict
	}
	return err
}

func (r *connectionRepo) find(ctx context.Context, filter bson.D) (*dao.Connection, error) {
	var c dao.Connection
	if err := r.coll().FindOne(ctx, filter).Decode(&c); err != nil {
		return nil, mapErr(err)
	}
	return &c, nil
}

func (r *connectionRepo) ByID(ctx context.Context, id string) (*dao.Connection, error) {
	return r.find(ctx, byID(id))
}

func (r *connectionRepo) ByPair(ctx context.Context, a, b string) (*dao.Connection, error) {
	return r.find(ctx, bson.D{{Key: "pairKey", Value: dao.PairKey(a, b)}})
}

func (r *connectionRepo) ListForUser(ctx context.Context, userID string, limit int) ([]dao.Connection, error) {
	filter := append(involving(userID), bson.E{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{dao.ConnectionPending, dao.ConnectionAccepted}}}})
	cur, err := r.coll().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []dao.Connection{}
	return out, cur.All(ctx, &out)
}

func (r *connectionRepo) pendingTo(id, addresseeID string) bson.D {
	return bson.D{{Key: "_id", Value: id}, {Key: "addresseeId", Value: addresseeID}, {Key: "status", Value: dao.ConnectionPending}}
}

func (r *connectionRepo) update(ctx context.Context, filter, set bson.D) error {
	res, err := r.coll().UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: set}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return apperrors.ErrRepoNotFound
	}
	return nil
}

func (r *connectionRepo) Accept(ctx context.Context, id, addresseeID string, at time.Time) error {
	return r.update(ctx, r.pendingTo(id, addresseeID), bson.D{{Key: "status", Value: dao.ConnectionAccepted}, {Key: "respondedAt", Value: at}})
}

func (r *connectionRepo) Decline(ctx context.Context, id, addresseeID string, at, until time.Time) error {
	return r.update(ctx, r.pendingTo(id, addresseeID),
		bson.D{{Key: "status", Value: dao.ConnectionDeclined}, {Key: "respondedAt", Value: at}, {Key: "expiresAt", Value: until}})
}

func (r *connectionRepo) Delete(ctx context.Context, id, userID string) error {
	res, err := r.coll().DeleteOne(ctx, append(bson.D{{Key: "_id", Value: id}}, involving(userID)...))
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apperrors.ErrRepoNotFound
	}
	return nil
}
