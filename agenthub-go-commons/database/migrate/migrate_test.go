//go:build integration

package migrate_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/database/migrate"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// TestUp_NonExistentMigrationsPath verifies that Up returns an error when the
// migrations path does not exist on the filesystem.
func TestUp_NonExistentMigrationsPath(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)

	err := migrate.Up(context.Background(), pool, "public", "/nonexistent/path/to/migrations")

	require.Error(t, err)
}

func TestUp_AppliesMigrationsInOrderAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	dir := t.TempDir()
	writeMigration(t, dir, "000002_insert_probe.up.sql", `
INSERT INTO migration_probe (id, name) VALUES (1, 'created');
`)
	writeMigration(t, dir, "000001_create_probe.up.sql", `
CREATE TABLE migration_probe (
	id integer PRIMARY KEY,
	name text NOT NULL
);
`)

	require.NoError(t, migrate.Up(ctx, pool, "public", dir))
	require.NoError(t, migrate.Up(ctx, pool, "public", dir))

	var name string
	require.NoError(t, pool.QueryRow(ctx, `SELECT name FROM public.migration_probe WHERE id = 1`).Scan(&name))
	require.Equal(t, "created", name)

	var version int64
	var dirty bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT version, dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, int64(2), version)
	require.False(t, dirty)
}

func TestUp_UsesCustomMigrationsTable(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	dir := t.TempDir()
	writeMigration(t, dir, "000001_create_probe.up.sql", `CREATE TABLE custom_table_probe (id integer PRIMARY KEY);`)

	require.NoError(t, migrate.Up(ctx, pool, "public", dir, "custom_migrations"))

	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.custom_migrations') IS NOT NULL`).Scan(&exists))
	require.True(t, exists)
}

func TestDown_RollsBackAllMigrations(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	dir := t.TempDir()
	writeMigration(t, dir, "000001_create_probe.up.sql", `CREATE TABLE rollback_probe (id integer PRIMARY KEY, name text NOT NULL);`)
	writeMigration(t, dir, "000002_insert_probe.up.sql", `INSERT INTO rollback_probe (id, name) VALUES (1, 'created');`)
	writeMigration(t, dir, "000002_insert_probe.down.sql", `DELETE FROM rollback_probe WHERE id = 1;`)
	writeMigration(t, dir, "000001_create_probe.down.sql", `DROP TABLE rollback_probe;`)

	require.NoError(t, migrate.Up(ctx, pool, "public", dir))
	require.NoError(t, migrate.Down(ctx, pool, "public", dir))

	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.rollback_probe') IS NOT NULL`).Scan(&exists))
	require.False(t, exists)

	var versionRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.schema_migrations`).Scan(&versionRows))
	require.Zero(t, versionRows)
}

func writeMigration(t *testing.T, dir, name, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600))
}
