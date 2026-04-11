package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/server"
	commonsmigratemulti "github.com/AgentHub-Studio/agenthub-go-commons/database/multitenant"
	commonsmigrate "github.com/AgentHub-Studio/agenthub-go-commons/database/migrate"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Apply public schema migrations.
	slog.Info("running public schema migrations")
	if err := commonsmigrate.Up(ctx, pool, "public", "/migrations/public"); err != nil {
		slog.Error("failed to run public migrations", "err", err)
		os.Exit(1)
	}

	// Apply per-tenant schema migrations to all ACTIVE tenants.
	slog.Info("running tenant schema migrations")
	if err := commonsmigratemulti.MigrateAllTenants(ctx, pool, "/migrations/schemas"); err != nil {
		slog.Error("failed to run tenant migrations", "err", err)
		os.Exit(1)
	}

	// Apply ah_core schema migrations (global, runs once).
	// Non-fatal: if the migration directory does not exist in this build, skip.
	slog.Info("running ah_core schema migrations")
	if err := commonsmigrate.Up(ctx, pool, "ah_core", "/migrations/ah_core"); err != nil {
		slog.Warn("ah_core migrations skipped or failed — platform tools may be unavailable", "err", err)
	}

	srv := server.New(cfg, pool)

	httpServer := &http.Server{
		Addr:        ":" + cfg.Port,
		Handler:     srv,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout must be 0 for SSE endpoints — the agentic runner
		// streams events over long-lived connections. Heartbeats (15s) and
		// context cancellation handle stale connections instead.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "err", err)
	}

	slog.Info("server stopped")
}

func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}
