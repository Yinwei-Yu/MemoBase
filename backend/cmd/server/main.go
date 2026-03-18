package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"memobase/backend/internal/api"
	"memobase/backend/internal/config"
	"memobase/backend/internal/core"
)

func main() {
	cfg := config.Load()
	logger := api.NewLogger(cfg.AppEnv)
	app, err := core.New(cfg, logger)
	if err != nil {
		panic(fmt.Errorf("boot failed: %w", err))
	}
	defer app.Close()

	r := api.NewServer(app)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server_started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("server_stopped")
}
