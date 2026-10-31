package mongorepo

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// EntityCollections maps an entity type to its collection.
var EntityCollections = map[string]string{
	entitytype.Habit:    constants.CollHabits,
	entitytype.DailyLog: constants.CollDailyLogs,
	entitytype.Workout:  constants.CollWorkouts,
	entitytype.Settings: constants.CollSettings,
	entitytype.Period:   constants.CollPeriods,
}

func isDup(err error) bool { return mongo.IsDuplicateKeyError(err) }

// mapErr converts driver errors to the typed repository errors.
func mapErr(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return apperrors.ErrRepoNotFound
	}
	return err
}

func byID(id string) bson.D { return bson.D{{Key: "_id", Value: id}} }

func asc(keys ...string) bson.D {
	d := bson.D{}
	for _, k := range keys {
		d = append(d, bson.E{Key: k, Value: 1})
	}
	return d
}

// EnsureIndexes creates all required indexes (idempotent). ddl/mongodb/indexes.js documents the same set.
func EnsureIndexes(ctx context.Context, db drivers.IMongoDB) error {
	d := db.Database()
	idx := func(coll string, models ...mongo.IndexModel) error {
		_, err := d.Collection(coll).Indexes().CreateMany(ctx, models)
		return err
	}
	// Unique email, plus unique phone only for users that have one (legacy users have no phone field).
	if err := idx(constants.CollUsers,
		mongo.IndexModel{Keys: asc("emailLower"), Options: options.Index().SetUnique(true)},
		mongo.IndexModel{Keys: asc("phone"), Options: options.Index().SetUnique(true).
			SetPartialFilterExpression(bson.D{{Key: "phone", Value: bson.D{{Key: "$type", Value: "string"}}}})}); err != nil {
		return err
	}
	if err := idx(constants.CollMemberships,
		mongo.IndexModel{Keys: asc("sharedSpaceId", "userId"), Options: options.Index().SetUnique(true)},
		mongo.IndexModel{Keys: asc("userId")}); err != nil {
		return err
	}
	for _, c := range EntityCollections {
		models := []mongo.IndexModel{{Keys: asc("spaceId", "seq")}}
		switch c {
		case constants.CollDailyLogs:
			models = append(models,
				mongo.IndexModel{Keys: asc("spaceId", "data.habitId", "data.localDate"), Options: options.Index().SetUnique(true)},
				mongo.IndexModel{Keys: asc("spaceId", "data.localDate")})
		case constants.CollWorkouts:
			models = append(models, mongo.IndexModel{Keys: asc("spaceId", "data.localDate")})
		case constants.CollPeriods:
			models = append(models, mongo.IndexModel{Keys: asc("spaceId", "data.startDate")})
		}
		if err := idx(c, models...); err != nil {
			return err
		}
	}
	if err := idx(constants.CollOps,
		mongo.IndexModel{Keys: asc("createdAt"), Options: options.Index().SetExpireAfterSeconds(int32(constants.OpRetention.Seconds()))}); err != nil {
		return err
	}
	if err := idx(constants.CollOTP,
		mongo.IndexModel{Keys: asc("expiresAt"), Options: options.Index().SetExpireAfterSeconds(0)},
		mongo.IndexModel{Keys: asc("email", "createdAt")}); err != nil {
		return err
	}
	return idx(constants.CollSessions,
		mongo.IndexModel{Keys: asc("tokenHash"), Options: options.Index().SetUnique(true)},
		mongo.IndexModel{Keys: asc("familyId")},
		mongo.IndexModel{Keys: asc("expiresAt"), Options: options.Index().SetExpireAfterSeconds(0)})
}
