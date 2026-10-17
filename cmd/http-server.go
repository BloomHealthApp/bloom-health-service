// Command http-server runs the Bubu Bloom REST + sync API.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bubu-bloom/service/pkg/application/httpserver"
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
	defer cleanup()
	di.AppLogic = app
	cfg, log := app.GetConfig(), app.GetLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.StoreKind == constants.StoreMemory && cfg.OwnerEmail != "" && cfg.ViewerEmail != "" {
		// Convenience for local development: the in-memory store starts empty on every run.
		err := app.GetProvisionService().Provision(ctx, serviceBos.ProvisionOptions{
			Owner:    serviceBos.Person{Email: cfg.OwnerEmail, Name: cfg.OwnerName},
			Viewer:   serviceBos.Person{Email: cfg.ViewerEmail, Name: cfg.ViewerName},
			Timezone: cfg.DefaultTZ,
		})
		if err != nil {
			log.Error(ctx, "dev seed failed", map[string]any{"err": err.Error()})
			cleanup()
			os.Exit(1)
		}
	}

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: httpserver.NewRouter(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info(ctx, "listening", map[string]any{"port": cfg.Port, "env": cfg.ActiveEnv, "store": cfg.StoreKind})
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error(ctx, "server error", map[string]any{"err": err.Error()})
		cleanup()
		os.Exit(1)
	}
}
