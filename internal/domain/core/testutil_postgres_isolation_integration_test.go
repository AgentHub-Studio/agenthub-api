//go:build integration

package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_TestutilPostgresDatabasesAreIsolated(t *testing.T) {
	ctx := context.Background()
	first := testutil.NewPostgresContainer(t)
	second := testutil.NewPostgresContainer(t)

	var firstDatabase string
	require.NoError(t, first.QueryRow(ctx, "SELECT current_database()").Scan(&firstDatabase))
	var secondDatabase string
	require.NoError(t, second.QueryRow(ctx, "SELECT current_database()").Scan(&secondDatabase))
	require.NotEqual(t, firstDatabase, secondDatabase)

	_, err := first.Exec(ctx, "CREATE TABLE fixture_isolation_probe (id integer PRIMARY KEY)")
	require.NoError(t, err)

	var relation *string
	require.NoError(t, second.QueryRow(ctx, "SELECT to_regclass('public.fixture_isolation_probe')").Scan(&relation))
	require.Nil(t, relation)
}
