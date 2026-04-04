package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/database"
)

// TestAcquireWithTenant_NoTenantInContext verifies that calling AcquireWithTenant
// with a context that carries no tenantID returns an error before ever touching
// the pool, so passing nil as the pool is safe here.
func TestAcquireWithTenant_NoTenantInContext(t *testing.T) {
	ctx := context.Background() // no tenantID set

	conn, release, err := database.AcquireWithTenant(ctx, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenantID not found")
	assert.Nil(t, conn)
	assert.Nil(t, release)
}
