package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/bubu-bloom/service/internal/model"
)

// Mongo is the MongoDB Atlas backed Store.
type Mongo struct {
	db *mongo.Database
}

var entityCollections = map[string]string{
	model.TypeHabit:    "habits",
	model.TypeDailyLog: "dailyHabitLogs",
	model.TypeWorkout:  "workoutSessions",
	model.TypeSettings: "settings",
}

func NewMongo(ctx context.Context, uri, dbName string) (*Mongo, error) {
	c, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	m := &Mongo{db: c.Database(dbName)}
	if err := m.Ping(ctx); err != nil {
		return nil, err
	}
	if err := m.EnsureIndexes(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Mongo) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.db.Client().Ping(ctx, nil)
}

// EnsureIndexes creates all required indexes (idempotent).
func (m *Mongo) EnsureIndexes(ctx context.Context) error {
	idx := func(coll string, models ...mongo.IndexModel) error {
		_, err := m.db.Collection(coll).Indexes().CreateMany(ctx, models)
		return err
	}
	asc := func(keys ...string) bson.D {
		d := bson.D{}
		for _, k := range keys {
			d = append(d, bson.E{Key: k, Value: 1})
		}
		return d
	}
	if err := idx("users", mongo.IndexModel{Keys: asc("emailLower"), Options: options.Index().SetUnique(true)}); err != nil {
		return err
	}
	if err := idx("memberships",
		mongo.IndexModel{Keys: asc("sharedSpaceId", "userId"), Options: options.Index().SetUnique(true)},
		mongo.IndexModel{Keys: asc("userId")}); err != nil {
		return err
	}
	for _, c := range entityCollections {
		models := []mongo.IndexModel{{Keys: asc("spaceId", "seq")}}
		switch c {
		case "dailyHabitLogs":
			models = append(models,
				mongo.IndexModel{Keys: asc("spaceId", "data.habitId", "data.localDate"), Options: options.Index().SetUnique(true)},
				mongo.IndexModel{Keys: asc("spaceId", "data.localDate")})
		case "workoutSessions":
			models = append(models, mongo.IndexModel{Keys: asc("spaceId", "data.localDate")})
		}
		if err := idx(c, models...); err != nil {
			return err
		}
	}
	if err := idx("appliedOperations",
		mongo.IndexModel{Keys: asc("createdAt"), Options: options.Index().SetExpireAfterSeconds(60 * 60 * 24 * 60)}); err != nil {
		return err
	}
	if err := idx("otpChallenges",
		mongo.IndexModel{Keys: asc("expiresAt"), Options: options.Index().SetExpireAfterSeconds(0)}); err != nil {
		return err
	}
	return idx("refreshSessions",
		mongo.IndexModel{Keys: asc("tokenHash"), Options: options.Index().SetUnique(true)},
		mongo.IndexModel{Keys: asc("familyId")},
		mongo.IndexModel{Keys: asc("expiresAt"), Options: options.Index().SetExpireAfterSeconds(0)})
}

func isDup(err error) bool { return mongo.IsDuplicateKeyError(err) }

func mapErr(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}
	return err
}

// ---- users / spaces / memberships ----

type userDoc struct {
	ID          string    `bson:"_id"`
	Email       string    `bson:"email"`
	EmailLower  string    `bson:"emailLower"`
	DisplayName string    `bson:"displayName"`
	Avatar      *string   `bson:"avatar"`
	Timezone    string    `bson:"timezone"`
	Theme       string    `bson:"theme"`
	CreatedAt   time.Time `bson:"createdAt"`
	UpdatedAt   time.Time `bson:"updatedAt"`
}

func (d userDoc) model() *model.User {
	return &model.User{ID: d.ID, Email: d.Email, DisplayName: d.DisplayName, Avatar: d.Avatar,
		Timezone: d.Timezone, Theme: d.Theme, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

func (m *Mongo) findUser(ctx context.Context, filter bson.D) (*model.User, error) {
	var d userDoc
	if err := m.db.Collection("users").FindOne(ctx, filter).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	return d.model(), nil
}

func (m *Mongo) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	return m.findUser(ctx, bson.D{{Key: "emailLower", Value: strings.ToLower(email)}})
}

func (m *Mongo) UserByID(ctx context.Context, id string) (*model.User, error) {
	return m.findUser(ctx, bson.D{{Key: "_id", Value: id}})
}

func (m *Mongo) SaveUser(ctx context.Context, u *model.User) error {
	d := userDoc{ID: u.ID, Email: u.Email, EmailLower: strings.ToLower(u.Email), DisplayName: u.DisplayName,
		Avatar: u.Avatar, Timezone: u.Timezone, Theme: u.Theme, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt}
	_, err := m.db.Collection("users").ReplaceOne(ctx, bson.D{{Key: "_id", Value: u.ID}}, d, options.Replace().SetUpsert(true))
	if isDup(err) {
		return ErrConflict
	}
	return err
}

type spaceDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	CreatedBy string    `bson:"createdBy"`
	CreatedAt time.Time `bson:"createdAt"`
}

func (m *Mongo) SpaceByID(ctx context.Context, id string) (*model.Space, error) {
	var d spaceDoc
	if err := m.db.Collection("sharedSpaces").FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	return &model.Space{ID: d.ID, Name: d.Name, CreatedBy: d.CreatedBy, CreatedAt: d.CreatedAt}, nil
}

func (m *Mongo) SaveSpace(ctx context.Context, s *model.Space) error {
	d := spaceDoc{ID: s.ID, Name: s.Name, CreatedBy: s.CreatedBy, CreatedAt: s.CreatedAt}
	_, err := m.db.Collection("sharedSpaces").ReplaceOne(ctx, bson.D{{Key: "_id", Value: s.ID}}, d, options.Replace().SetUpsert(true))
	return err
}

type membershipDoc struct {
	ID        string     `bson:"_id"`
	SpaceID   string     `bson:"sharedSpaceId"`
	UserID    string     `bson:"userId"`
	Role      model.Role `bson:"role"`
	CreatedAt time.Time  `bson:"createdAt"`
}

func (d membershipDoc) model() model.Membership {
	return model.Membership{ID: d.ID, SpaceID: d.SpaceID, UserID: d.UserID, Role: d.Role, CreatedAt: d.CreatedAt}
}

func (m *Mongo) MembershipForUser(ctx context.Context, userID string) (*model.Membership, error) {
	var d membershipDoc
	if err := m.db.Collection("memberships").FindOne(ctx, bson.D{{Key: "userId", Value: userID}}).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	mm := d.model()
	return &mm, nil
}

func (m *Mongo) MembersOfSpace(ctx context.Context, spaceID string) ([]model.Membership, error) {
	cur, err := m.db.Collection("memberships").Find(ctx, bson.D{{Key: "sharedSpaceId", Value: spaceID}})
	if err != nil {
		return nil, err
	}
	var docs []membershipDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]model.Membership, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.model())
	}
	return out, nil
}

func (m *Mongo) SaveMembership(ctx context.Context, mm *model.Membership) error {
	d := membershipDoc{ID: mm.ID, SpaceID: mm.SpaceID, UserID: mm.UserID, Role: mm.Role, CreatedAt: mm.CreatedAt}
	_, err := m.db.Collection("memberships").ReplaceOne(ctx, bson.D{{Key: "_id", Value: mm.ID}}, d, options.Replace().SetUpsert(true))
	if isDup(err) {
		return ErrConflict
	}
	return err
}

// ---- entities ----

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

// normalize converts BSON values (bson.M / bson.A / bson.D) into plain JSON-style Go values.
func normalize(in bson.M) (map[string]any, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	return out, json.Unmarshal(b, &out)
}

func (d entityDoc) model(typ string) (*model.Entity, error) {
	data, err := normalize(d.Data)
	if err != nil {
		return nil, err
	}
	return &model.Entity{ID: d.EID, Type: typ, SpaceID: d.SpaceID, Version: d.Version, Seq: d.Seq,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, DeletedAt: d.DeletedAt, Data: data}, nil
}

func (m *Mongo) coll(typ string) (*mongo.Collection, error) {
	c, ok := entityCollections[typ]
	if !ok {
		return nil, errors.New("unknown entity type " + typ)
	}
	return m.db.Collection(c), nil
}

func (m *Mongo) nextSeq(ctx context.Context, spaceID string) (int64, error) {
	var out struct {
		Seq int64 `bson:"seq"`
	}
	err := m.db.Collection("counters").FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: spaceID}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "seq", Value: 1}}}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&out)
	return out.Seq, err
}

func (m *Mongo) GetEntity(ctx context.Context, spaceID, typ, id string) (*model.Entity, error) {
	c, err := m.coll(typ)
	if err != nil {
		return nil, err
	}
	var d entityDoc
	if err := c.FindOne(ctx, bson.D{{Key: "_id", Value: docID(spaceID, id)}}).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	return d.model(typ)
}

func (m *Mongo) ListEntities(ctx context.Context, spaceID, typ string, q ListQuery) ([]model.Entity, error) {
	c, err := m.coll(typ)
	if err != nil {
		return nil, err
	}
	filter := bson.D{{Key: "spaceId", Value: spaceID}}
	if !q.IncludeDeleted {
		filter = append(filter, bson.E{Key: "deletedAt", Value: nil})
	}
	if q.From != "" || q.To != "" {
		r := bson.D{}
		if q.From != "" {
			r = append(r, bson.E{Key: "$gte", Value: q.From})
		}
		if q.To != "" {
			r = append(r, bson.E{Key: "$lte", Value: q.To})
		}
		filter = append(filter, bson.E{Key: "data.localDate", Value: r})
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

func toEntities(docs []entityDoc, typ string) ([]model.Entity, error) {
	out := make([]model.Entity, 0, len(docs))
	for _, d := range docs {
		e, err := d.model(typ)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, nil
}

func (m *Mongo) SaveEntity(ctx context.Context, e *model.Entity) error {
	c, err := m.coll(e.Type)
	if err != nil {
		return err
	}
	seq, err := m.nextSeq(ctx, e.SpaceID)
	if err != nil {
		return err
	}
	doc := entityDoc{ID: docID(e.SpaceID, e.ID), EID: e.ID, SpaceID: e.SpaceID, Version: e.Version, Seq: seq,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt, Data: bson.M(e.Data)}
	if e.Version == 1 {
		if _, err := c.InsertOne(ctx, doc); err != nil {
			if isDup(err) {
				return ErrConflict
			}
			return err
		}
	} else {
		res, err := c.ReplaceOne(ctx, bson.D{{Key: "_id", Value: doc.ID}, {Key: "version", Value: e.Version - 1}}, doc)
		if err != nil {
			if isDup(err) {
				return ErrConflict
			}
			return err
		}
		if res.MatchedCount == 0 {
			return ErrConflict
		}
	}
	e.Seq = seq
	return nil
}

func (m *Mongo) Changes(ctx context.Context, spaceID string, after int64, limit int) ([]model.Entity, error) {
	var all []model.Entity
	for _, typ := range model.EntityTypes {
		c, _ := m.coll(typ)
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
	// merge-sort by seq across collections (seq is unique per space)
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].Seq < all[j-1].Seq; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// ---- operations ----

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

func (m *Mongo) GetOp(ctx context.Context, id string) (*model.OpRecord, error) {
	var d opDoc
	if err := m.db.Collection("appliedOperations").FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	r := &model.OpRecord{OperationID: d.ID, SpaceID: d.SpaceID, Status: d.Status, Reason: d.Reason, Resolved: d.Resolved, CreatedAt: d.CreatedAt}
	if d.Entity != nil {
		e, err := d.Entity.model(d.EntityTyp)
		if err != nil {
			return nil, err
		}
		r.Entity = e
	}
	return r, nil
}

func (m *Mongo) SaveOp(ctx context.Context, r *model.OpRecord) error {
	d := opDoc{ID: r.OperationID, SpaceID: r.SpaceID, Status: r.Status, Reason: r.Reason, Resolved: r.Resolved, CreatedAt: r.CreatedAt}
	if r.Entity != nil {
		d.EntityTyp = r.Entity.Type
		d.Entity = &entityDoc{ID: docID(r.Entity.SpaceID, r.Entity.ID), EID: r.Entity.ID, SpaceID: r.Entity.SpaceID,
			Version: r.Entity.Version, Seq: r.Entity.Seq, CreatedAt: r.Entity.CreatedAt, UpdatedAt: r.Entity.UpdatedAt,
			DeletedAt: r.Entity.DeletedAt, Data: bson.M(r.Entity.Data)}
	}
	if _, err := m.db.Collection("appliedOperations").InsertOne(ctx, d); err != nil {
		if isDup(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

// ---- OTP ----

type otpDoc struct {
	Email     string    `bson:"_id"`
	CodeHash  string    `bson:"codeHash"`
	ExpiresAt time.Time `bson:"expiresAt"`
	Attempts  int       `bson:"attempts"`
}

func (m *Mongo) SaveOTP(ctx context.Context, c *model.OTPChallenge) error {
	d := otpDoc{Email: strings.ToLower(c.Email), CodeHash: c.CodeHash, ExpiresAt: c.ExpiresAt, Attempts: c.Attempts}
	_, err := m.db.Collection("otpChallenges").ReplaceOne(ctx, bson.D{{Key: "_id", Value: d.Email}}, d, options.Replace().SetUpsert(true))
	return err
}

func (m *Mongo) GetOTP(ctx context.Context, email string) (*model.OTPChallenge, error) {
	var d otpDoc
	if err := m.db.Collection("otpChallenges").FindOne(ctx, bson.D{{Key: "_id", Value: strings.ToLower(email)}}).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	return &model.OTPChallenge{Email: d.Email, CodeHash: d.CodeHash, ExpiresAt: d.ExpiresAt, Attempts: d.Attempts}, nil
}

func (m *Mongo) AddOTPAttempt(ctx context.Context, email string) (int, error) {
	var d otpDoc
	err := m.db.Collection("otpChallenges").FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: strings.ToLower(email)}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "attempts", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&d)
	if err != nil {
		return 0, mapErr(err)
	}
	return d.Attempts, nil
}

func (m *Mongo) DeleteOTP(ctx context.Context, email string) error {
	_, err := m.db.Collection("otpChallenges").DeleteOne(ctx, bson.D{{Key: "_id", Value: strings.ToLower(email)}})
	return err
}

// ---- refresh sessions ----

type sessionDoc struct {
	ID        string     `bson:"_id"`
	FamilyID  string     `bson:"familyId"`
	UserID    string     `bson:"userId"`
	TokenHash string     `bson:"tokenHash"`
	ExpiresAt time.Time  `bson:"expiresAt"`
	UsedAt    *time.Time `bson:"usedAt"`
	RevokedAt *time.Time `bson:"revokedAt"`
	CreatedAt time.Time  `bson:"createdAt"`
}

func (m *Mongo) SaveSession(ctx context.Context, s *model.RefreshSession) error {
	d := sessionDoc(*s)
	_, err := m.db.Collection("refreshSessions").InsertOne(ctx, d)
	if isDup(err) {
		return ErrConflict
	}
	return err
}

func (m *Mongo) SessionByHash(ctx context.Context, hash string) (*model.RefreshSession, error) {
	var d sessionDoc
	if err := m.db.Collection("refreshSessions").FindOne(ctx, bson.D{{Key: "tokenHash", Value: hash}}).Decode(&d); err != nil {
		return nil, mapErr(err)
	}
	s := model.RefreshSession(d)
	return &s, nil
}

func (m *Mongo) MarkSessionUsed(ctx context.Context, id string) (bool, error) {
	res, err := m.db.Collection("refreshSessions").UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "usedAt", Value: nil}, {Key: "revokedAt", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "usedAt", Value: time.Now().UTC()}}}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

func (m *Mongo) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := m.db.Collection("refreshSessions").UpdateMany(ctx,
		bson.D{{Key: "familyId", Value: familyID}, {Key: "revokedAt", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "revokedAt", Value: time.Now().UTC()}}}})
	return err
}
