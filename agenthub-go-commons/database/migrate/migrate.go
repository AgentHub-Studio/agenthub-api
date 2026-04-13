package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Up applies all pending migrations from migrationsPath to the given schema.
// migrationsPath is a filesystem path e.g. "migrations/public".
// ErrNoChange is not treated as an error.
//
// When migrationsTable is empty, "schema_migrations" is used (default).
// Callers that run multiple independent migration sequences against the same
// schema (e.g. tenant schemas + ah_core seed migrations) MUST pass a distinct
// table name to avoid version tracking conflicts.
func Up(ctx context.Context, pool *pgxpool.Pool, schema, migrationsPath string, opts ...string) error {
	migrationsTable := "schema_migrations"
	if len(opts) > 0 && opts[0] != "" {
		migrationsTable = opts[0]
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	// Obtain a dedicated connection and pin search_path to the target schema.
	// Pool connections may carry a stale search_path from AcquireWithTenant
	// (e.g. "ah_test, public"). Without an explicit SET, golang-migrate would
	// run migration SQL against the wrong schema, causing IF NOT EXISTS checks
	// to find tables from other schemas and subsequent CREATE INDEX statements
	// to fail with "column does not exist".
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire connection for schema %q: %w", schema, err)
	}
	defer conn.Close()

	// Include public so extension types (e.g. vector) remain accessible.
	// New objects are still created in the tenant schema (first in path).
	// IF NOT EXISTS checks also only look in the first schema, so tables in
	// public (e.g. a legacy chat_session) do not interfere.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		return fmt.Errorf("migrate: set search_path for schema %q: %w", schema, err)
	}

	driver, err := postgres.WithConnection(ctx, conn, &postgres.Config{
		SchemaName:      schema,
		MigrationsTable: migrationsTable,
	})
	if err != nil {
		return fmt.Errorf("migrate: create driver for schema %q: %w", schema, err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate: initialize for schema %q: %w", schema, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up schema %q: %w", schema, err)
	}

	return nil
}

// Down rolls back all migrations for the given schema.
func Down(ctx context.Context, pool *pgxpool.Pool, schema, migrationsPath string) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{
		SchemaName:      schema,
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("migrate: create driver for schema %q: %w", schema, err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate: initialize for schema %q: %w", schema, err)
	}
	defer m.Close()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: down schema %q: %w", schema, err)
	}
	return nil
}
