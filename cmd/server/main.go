package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bubu-bloom/service/internal/auth"
	"github.com/bubu-bloom/service/internal/config"
	"github.com/bubu-bloom/service/internal/httpapi"
	"github.com/bubu-bloom/service/internal/seed"
	"github.com/bubu-bloom/service/internal/service"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("invalid configuration", "err", err.Error())
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := cfg.OpenStore(ctx)
	if err != nil {
		log.Error("store connection failed", "err", err.Error())
		os.Exit(1)
	}
	if cfg.StoreKind == "memory" && cfg.OwnerEmail != "" && cfg.ViewerEmail != "" {
		// Convenience for local dev: the in-memory store starts empty on every run.
		err := seed.Provision(ctx, st, seed.Options{Owner: seed.Person{Email: cfg.OwnerEmail, Name: cfg.OwnerName},
			Viewer: seed.Person{Email: cfg.ViewerEmail, Name: cfg.ViewerName}, Timezone: cfg.DefaultTZ}, time.Now().UTC())
		if err != nil {
			log.Error("dev seed failed", "err", err.Error())
			os.Exit(1)
		}
	}
	authSvc := auth.New(st, cfg.NewMailer(log), cfg.AccessSecret, cfg.Issuer)
	api := httpapi.NewServer(authSvc, service.New(st), st, log, cfg.TrustProxy)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("listening", "port", cfg.Port, "env", cfg.Env, "store", cfg.StoreKind)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", "err", err.Error())
		os.Exit(1)
	}
}
