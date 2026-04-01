package tenant_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

func TestFromContext_ReturnsID(t *testing.T) {
	ctx := tenant.NewContext(context.Background(), "my-company")
	assert.Equal(t, "my-company", tenant.FromContext(ctx))
}

func TestFromContext_EmptyWhenMissing(t *testing.T) {
	assert.Empty(t, tenant.FromContext(context.Background()))
}

func TestNewContext_OverridesParent(t *testing.T) {
	ctx := tenant.NewContext(context.Background(), "first-tenant")
	ctx2 := tenant.NewContext(ctx, "second-tenant")
	assert.Equal(t, "second-tenant", tenant.FromContext(ctx2))
	// Parent context is unchanged.
	assert.Equal(t, "first-tenant", tenant.FromContext(ctx))
}
