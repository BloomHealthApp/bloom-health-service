package di

import (
	"context"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/drivers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo/memory"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
)

// Repositories bundles every repository plus the health pinger of the selected backend.
type Repositories struct {
	Users        mongorepo.IUserRepo
	Spaces       mongorepo.ISpaceRepo
	Memberships  mongorepo.IMembershipRepo
	Entities     mongorepo.IEntityRepo
	Ops          mongorepo.IOpRepo
	OTPs         mongorepo.IOTPRepo
	Sessions     mongorepo.ISessionRepo
	ChatSessions mongorepo.IChatSessionRepo
	ChatMessages mongorepo.IChatMessageRepo
	Pinger       drivers.IPinger
}

// NewMemoryRepositories returns fresh in-memory repositories (tests and STORE=memory).
func NewMemoryRepositories() *Repositories {
	return &Repositories{Users: memory.NewUserRepo(), Spaces: memory.NewSpaceRepo(), Memberships: memory.NewMembershipRepo(),
		Entities: memory.NewEntityRepo(), Ops: memory.NewOpRepo(), OTPs: memory.NewOTPRepo(), Sessions: memory.NewSessionRepo(),
		ChatSessions: memory.NewChatSessionRepo(), ChatMessages: memory.NewChatMessageRepo(), Pinger: memory.Pinger{}}
}

// NewRepositories builds the repositories for cfg.StoreKind. For MongoDB it connects and pings, then
// ensures indexes in the background; the returned cleanup disconnects.
func NewRepositories(cfg *config.Store, log logger.ILogger) (*Repositories, func(), error) {
	if cfg.StoreKind == constants.StoreMemory {
		return NewMemoryRepositories(), func() {}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	started := time.Now()
	db, err := drivers.NewMongoDB(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		return nil, nil, err
	}
	log.Info(ctx, "mongo connected", map[string]any{"ms": time.Since(started).Milliseconds()})
	// Index creation is idempotent and the indexes already exist after the first start, so a cold start (Vercel scales
	// to zero) must not make its first request wait for it.
	go func() {
		ictx, icancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer icancel()
		began := time.Now()
		if err := mongorepo.EnsureIndexes(ictx, db); err != nil {
			log.Error(ictx, "ensure indexes failed", map[string]any{"err": err.Error()})
			return
		}
		log.Info(ictx, "indexes ensured", map[string]any{"ms": time.Since(began).Milliseconds()})
	}()
	cleanup := func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Close(c)
	}
	return &Repositories{
		Users: mongorepo.NewUserRepo(db, log), Spaces: mongorepo.NewSpaceRepo(db, log),
		Memberships: mongorepo.NewMembershipRepo(db, log), Entities: mongorepo.NewEntityRepo(db, log),
		Ops: mongorepo.NewOpRepo(db, log), OTPs: mongorepo.NewOTPRepo(db, log), Sessions: mongorepo.NewSessionRepo(db, log),
		ChatSessions: mongorepo.NewChatSessionRepo(db, log), ChatMessages: mongorepo.NewChatMessageRepo(db, log),
		Pinger: db,
	}, cleanup, nil
}
