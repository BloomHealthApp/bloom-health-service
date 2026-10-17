// Command seed provisions the owner/viewer users, shared space and default habits in MongoDB. Safe to re-run.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bubu-bloom/service/internal/config"
	"github.com/bubu-bloom/service/internal/seed"
	"github.com/bubu-bloom/service/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if cfg.StoreKind != "mongo" {
		fmt.Fprintln(os.Stderr, "seed requires STORE=mongo (the in-memory store is seeded automatically at server start when OWNER_EMAIL and VIEWER_EMAIL are set)")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := store.NewMongo(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mongo:", err)
		os.Exit(1)
	}
	err = seed.Provision(ctx, st, seed.Options{Owner: seed.Person{Email: cfg.OwnerEmail, Name: cfg.OwnerName},
		Viewer: seed.Person{Email: cfg.ViewerEmail, Name: cfg.ViewerName}, Timezone: cfg.DefaultTZ}, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Println("seed complete")
}
