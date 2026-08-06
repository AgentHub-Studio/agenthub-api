package migrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultMigrationsTable = "schema_migrations"

var migrationFilePattern = regexp.MustCompile(`^([0-9]+)_.+\.(up|down)\.sql$`)

type migrationFile struct {
	version int64
	path    string
}

// Up applies all pending migrations from migrationsPath to the given schema.
// migrationsPath is a filesystem path e.g. "migrations/public".
// ErrNoChange is not treated as an error.
//
// When migrationsTable is empty, "schema_migrations" is used (default).
// Callers that run multiple independent migration sequences against the same
// schema (e.g. tenant schemas + ah_core seed migrations) MUST pass a distinct
// table name to avoid version tracking conflicts.
func Up(ctx context.Context, pool *pgxpool.Pool, schema, migrationsPath string, opts ...string) error {
	migrationsTable := tableNameFromOptions(opts...)
	conn, release, err := acquireMigrationConnection(ctx, pool, schema, migrationsTable)
	if err != nil {
		return err
	}
	defer release()

	migrations, err := discoverMigrationFiles(migrationsPath, "up")
	if err != nil {
		return fmt.Errorf("migrate: discover up migrations for schema %q: %w", schema, err)
	}

	version, dirty, err := currentVersion(ctx, conn, schema, migrationsTable)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("migrate: schema %q is dirty at version %d", schema, version)
	}

	for _, migration := range migrations {
		if migration.version <= version {
			continue
		}
		if err := applyMigration(ctx, conn, schema, migrationsTable, migration, migration.version); err != nil {
			return fmt.Errorf("migrate: up schema %q version %d: %w", schema, migration.version, err)
		}
		version = migration.version
	}

	return nil
}

// Down rolls back all migrations for the given schema.
func Down(ctx context.Context, pool *pgxpool.Pool, schema, migrationsPath string) error {
	conn, release, err := acquireMigrationConnection(ctx, pool, schema, defaultMigrationsTable)
	if err != nil {
		return err
	}
	defer release()

	upMigrations, err := discoverMigrationFiles(migrationsPath, "up")
	if err != nil {
		return fmt.Errorf("migrate: discover up migrations for schema %q: %w", schema, err)
	}
	downMigrations, err := discoverMigrationFiles(migrationsPath, "down")
	if err != nil {
		return fmt.Errorf("migrate: discover down migrations for schema %q: %w", schema, err)
	}
	sort.Slice(upMigrations, func(i, j int) bool {
		return upMigrations[i].version > upMigrations[j].version
	})
	downByVersion := make(map[int64]migrationFile, len(downMigrations))
	for _, migration := range downMigrations {
		downByVersion[migration.version] = migration
	}

	version, dirty, err := currentVersion(ctx, conn, schema, defaultMigrationsTable)
	if err != nil {
		return err
	}
	if version < 0 {
		return nil
	}
	if dirty {
		return fmt.Errorf("migrate: schema %q is dirty at version %d", schema, version)
	}

	for index, applied := range upMigrations {
		if applied.version > version {
			continue
		}
		migration, ok := downByVersion[applied.version]
		if !ok {
			return fmt.Errorf("missing down migration for version %d", applied.version)
		}
		nextVersion := int64(-1)
		for _, candidate := range upMigrations[index+1:] {
			if candidate.version < applied.version {
				nextVersion = candidate.version
				break
			}
		}
		if err := applyMigration(ctx, conn, schema, defaultMigrationsTable, migration, nextVersion); err != nil {
			return fmt.Errorf("migrate: down schema %q version %d: %w", schema, migration.version, err)
		}
	}

	return nil
}

func tableNameFromOptions(opts ...string) string {
	if len(opts) > 0 && opts[0] != "" {
		return opts[0]
	}
	return defaultMigrationsTable
}

func acquireMigrationConnection(ctx context.Context, pool *pgxpool.Pool, schema, table string) (*pgxpool.Conn, func(), error) {
	quotedSchema, err := quoteIdentifier(schema)
	if err != nil {
		return nil, nil, fmt.Errorf("migrate: invalid schema name: %w", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("migrate: acquire connection for schema %q: %w", schema, err)
	}

	release := func() {
		conn.Release()
	}

	lockKey := schema + "." + table
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1)::bigint)`, lockKey); err != nil {
		release()
		return nil, nil, fmt.Errorf("migrate: acquire advisory lock for schema %q: %w", schema, err)
	}

	release = func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1)::bigint)`, lockKey)
		conn.Release()
	}

	if _, err := conn.Exec(ctx, "SET search_path TO "+quotedSchema+", public"); err != nil {
		release()
		return nil, nil, fmt.Errorf("migrate: set search_path for schema %q: %w", schema, err)
	}
	if err := ensureVersionTable(ctx, conn, schema, table); err != nil {
		release()
		return nil, nil, err
	}

	return conn, release, nil
}

func discoverMigrationFiles(migrationsPath, direction string) ([]migrationFile, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, err
	}

	seen := make(map[int64]string)
	migrations := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := migrationFilePattern.FindStringSubmatch(entry.Name())
		if len(matches) != 3 || matches[2] != direction {
			continue
		}

		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse version from %q: %w", entry.Name(), err)
		}
		if previous, exists := seen[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, previous, entry.Name())
		}
		seen[version] = entry.Name()
		migrations = append(migrations, migrationFile{
			version: version,
			path:    filepath.Join(migrationsPath, entry.Name()),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func ensureVersionTable(ctx context.Context, conn *pgxpool.Conn, schema, table string) error {
	qualifiedTable, err := qualifiedIdentifier(schema, table)
	if err != nil {
		return fmt.Errorf("migrate: invalid version table name: %w", err)
	}
	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+qualifiedTable+` (version bigint not null primary key, dirty boolean not null)`)
	if err != nil {
		return fmt.Errorf("migrate: ensure version table %s: %w", qualifiedTable, err)
	}
	return nil
}

func currentVersion(ctx context.Context, conn *pgxpool.Conn, schema, table string) (int64, bool, error) {
	qualifiedTable, err := qualifiedIdentifier(schema, table)
	if err != nil {
		return 0, false, fmt.Errorf("migrate: invalid version table name: %w", err)
	}

	var version int64
	var dirty bool
	err = conn.QueryRow(ctx, `SELECT version, dirty FROM `+qualifiedTable+` LIMIT 1`).Scan(&version, &dirty)
	if err == nil {
		return version, dirty, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return -1, false, nil
	}
	return 0, false, fmt.Errorf("migrate: read version table %s: %w", qualifiedTable, err)
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, schema, table string, migration migrationFile, cleanVersion int64) error {
	if err := setVersion(ctx, conn, schema, table, migration.version, true); err != nil {
		return err
	}

	sqlBytes, err := os.ReadFile(migration.path)
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(sqlBytes)) != "" {
		if _, err := conn.Exec(ctx, string(sqlBytes)); err != nil {
			return err
		}
	}

	return setVersion(ctx, conn, schema, table, cleanVersion, false)
}

func setVersion(ctx context.Context, conn *pgxpool.Conn, schema, table string, version int64, dirty bool) error {
	qualifiedTable, err := qualifiedIdentifier(schema, table)
	if err != nil {
		return fmt.Errorf("migrate: invalid version table name: %w", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin version update: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM `+qualifiedTable); err != nil {
		return fmt.Errorf("migrate: clear version table %s: %w", qualifiedTable, err)
	}
	if version >= 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO `+qualifiedTable+` (version, dirty) VALUES ($1, $2)`, version, dirty); err != nil {
			return fmt.Errorf("migrate: update version table %s: %w", qualifiedTable, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate: commit version update: %w", err)
	}
	return nil
}

func qualifiedIdentifier(schema, table string) (string, error) {
	quotedSchema, err := quoteIdentifier(schema)
	if err != nil {
		return "", err
	}
	quotedTable, err := quoteIdentifier(table)
	if err != nil {
		return "", err
	}
	return quotedSchema + "." + quotedTable, nil
}

func quoteIdentifier(identifier string) (string, error) {
	if strings.TrimSpace(identifier) == "" {
		return "", fmt.Errorf("identifier is empty")
	}
	if strings.ContainsRune(identifier, 0) {
		return "", fmt.Errorf("identifier contains NUL byte")
	}
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`, nil
}
