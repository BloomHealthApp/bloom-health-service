// Package drivers wraps infrastructure clients behind small interfaces so they can be mocked.
package drivers

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// IPinger checks backend reachability (used by /health).
type IPinger interface {
	Ping(ctx context.Context) error
}

// IMongoDB is the MongoDB connection used by the mongorepo repositories.
type IMongoDB interface {
	IPinger
	// Database returns the application database handle.
	Database() *mongo.Database
	// Close disconnects the client.
	Close(ctx context.Context) error
}

type mongoDB struct {
	client *mongo.Client
	db     *mongo.Database
}

// NewMongoDB connects to MongoDB and verifies the connection with a ping.
func NewMongoDB(ctx context.Context, uri, dbName string) (IMongoDB, error) {
	c, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	m := &mongoDB{client: c, db: c.Database(dbName)}
	if err := m.Ping(ctx); err != nil {
		_ = c.Disconnect(context.Background())
		return nil, err
	}
	return m, nil
}

func (m *mongoDB) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.client.Ping(ctx, nil)
}

func (m *mongoDB) Database() *mongo.Database { return m.db }

func (m *mongoDB) Close(ctx context.Context) error { return m.client.Disconnect(ctx) }
