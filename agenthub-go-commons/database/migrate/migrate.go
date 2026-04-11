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

	driver, err := postgres.WithInstance(db, &postgres.Config{
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
