//go:build integration

package mcp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_MCPServerConfigDefaultsToStdio(t *testing.T) {
	pool, _ := setupMCPTenantSchema(t)
	ctx := context.Background()

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET search_path TO ah_"+mcpIntegrationTenant)
	require.NoError(t, err)

	var transportType string
	err = conn.QueryRow(ctx,
		`INSERT INTO mcp_server_config (name, command)
		 VALUES ($1, $2)
		 RETURNING transport_type`,
		"default-stdio", "echo",
	).Scan(&transportType)
	require.NoError(t, err)
	assert.Equal(t, "stdio", transportType)
}
