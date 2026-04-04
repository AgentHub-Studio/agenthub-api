//go:build integration

package migrate_test

import (
	"context"
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
