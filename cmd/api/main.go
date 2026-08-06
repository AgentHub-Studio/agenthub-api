package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/server"
	commonsmigrate "github.com/AgentHub-Studio/agenthub-go-commons/database/migrate"
	commonsmigratemulti "github.com/AgentHub-Studio/agenthub-go-commons/database/multitenant"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-health" {
		os.Exit(runHealthCheck())
	}

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
	// "ah_core_seed_migrations" is a separate tracking table so that the ah_core
	// seed migrations (tools/skills/agents) do not conflict with the tenant
	// schema_migrations table that MigrateAllTenants manages for ah_core.
	// The migrate driver creates its tracking table before applying migration
	// files, so the target schema must already exist even though migration 000001
	// also declares CREATE SCHEMA IF NOT EXISTS.
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS ah_core`); err != nil {
		slog.Warn("ah_core schema ensure failed — platform tools may be unavailable", "err", err)
	} else if err := commonsmigrate.Up(ctx, pool, "ah_core", "/migrations/ah_core", "ah_core_seed_migrations"); err != nil {
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

	httpShutdownDone := make(chan error, 1)
	go func() {
		httpShutdownDone <- httpServer.Shutdown(shutdownCtx)
	}()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("application shutdown error", "err", err)
	}

	if err := <-httpShutdownDone; err != nil {
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

func runHealthCheck() int {
	endpoint, ok := healthCheckEndpoint(os.Getenv("PORT"))
	if !ok {
		return 1
	}

	client := http.Client{Timeout: 2 * time.Second}
	// #nosec G704 -- endpoint uses fixed loopback host and a validated numeric port from the environment.
	resp, err := client.Get(endpoint)
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 1
	}
	return 0
}

func healthCheckEndpoint(rawPort string) (string, bool) {
	trimmedPort := strings.TrimSpace(rawPort)
	if rawPort != "" && rawPort != trimmedPort {
		return "", false
	}
	port := trimmedPort
	if port == "" {
		port = "8081"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", false
	}
	endpoint := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", port),
		Path:   "/health",
	}
	return endpoint.String(), true
}
