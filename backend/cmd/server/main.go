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

	"flowmetrics/internal/auth"
	"flowmetrics/internal/config"
	"flowmetrics/internal/database"
	"flowmetrics/internal/handler"
	"flowmetrics/internal/repository"
	"flowmetrics/internal/seed"
	"flowmetrics/internal/service"
)

func main() {
	// `server healthcheck` lets the distroless container (no shell, no curl) probe itself.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		resp, err := (&http.Client{Timeout: 3 * time.Second}).Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Env == "production" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, db, err := database.Open(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())

	repos := repository.New(db)
	svc := service.New(repos, auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL), cfg.BcryptCost, cfg.RefreshTTL)

	if cfg.SeedOnStart {
		pw := os.Getenv("SEED_PASSWORD")
		if pw == "" {
			pw = "Password123!" // demo accounts only; never enable SEED_ON_START outside local development
		}
		if err := seed.Run(ctx, repos, pw, cfg.BcryptCost); err != nil {
			return err
		}
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.NewRouter(cfg, svc, func(ctx context.Context) error { return client.Ping(ctx, nil) }),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
