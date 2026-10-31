// Command http-server runs the Bloom Health REST + sync API.
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

	"github.com/BloomHealthApp/bloom-health-service/pkg/application/httpserver"
	"github.com/BloomHealthApp/bloom-health-service/pkg/di"
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
