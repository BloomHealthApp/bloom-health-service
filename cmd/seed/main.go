// Command seed provisions the owner/viewer users, shared space and default habits in MongoDB.
// It is idempotent and safe to re-run. Requires STORE=mongo, OWNER_EMAIL and VIEWER_EMAIL.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bubu-bloom/service/pkg/constants"
	"github.com/bubu-bloom/service/pkg/di"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
)

func main() {
	app, cleanup, err := di.InitializeApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "startup failed:", err)
		os.Exit(1)
	}
	cfg := app.GetConfig()
	if cfg.StoreKind != constants.StoreMongo {
		cleanup()
		fmt.Fprintln(os.Stderr, "seed requires STORE=mongo (the in-memory store is seeded automatically at server start when OWNER_EMAIL and VIEWER_EMAIL are set)")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err = app.GetProvisionService().Provision(ctx, serviceBos.ProvisionOptions{
		Owner:    serviceBos.Person{Email: cfg.OwnerEmail, Name: cfg.OwnerName},
		Viewer:   serviceBos.Person{Email: cfg.ViewerEmail, Name: cfg.ViewerName},
		Timezone: cfg.DefaultTZ,
	})
	cancel()
	cleanup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Println("seed complete")
}
