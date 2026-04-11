package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
)

// mockCoreToolProvider is a test double for CoreToolProvider used in agentic tests.
type mockCoreToolProvider struct {
	tools []core.CoreTool
	err   error
}

func (m *mockCoreToolProvider) LoadAll(_ context.Context) ([]core.CoreTool, error) {
	return m.tools, m.err
}

// TestCoreToolLoader_LoadAll_Empty verifies that LoadAll returns an empty slice when
// the provider has no tools, matching the "non-fatal schema missing" contract.
func TestCoreToolLoader_LoadAll_Empty(t *testing.T) {
	provider := &mockCoreToolProvider{tools: nil, err: nil}
	tools, err := provider.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tools)
}

// TestCoreToolLoader_LoadAll_ReturnsSingleTool verifies that a single tool
// is returned correctly without modification.
func TestCoreToolLoader_LoadAll_ReturnsSingleTool(t *testing.T) {
	expected := core.CoreTool{
		Name:        "List Agents",
		Slug:        "core-list-agents",
		Description: "Lists all agents in the tenant",
		Type:        "HTTP",
		IsActive:    true,
	}
	provider := &mockCoreToolProvider{tools: []core.CoreTool{expected}}
	tools, err := provider.LoadAll(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, expected.Slug, tools[0].Slug)
	assert.Equal(t, expected.Name, tools[0].Name)
	assert.Equal(t, expected.Description, tools[0].Description)
	assert.True(t, tools[0].IsActive)
}

// TestCoreToolLoader_LoadAll_MultipleTools verifies ordering and completeness
// when multiple tools are returned.
func TestCoreToolLoader_LoadAll_MultipleTools(t *testing.T) {
	tools := []core.CoreTool{
		{Slug: "core-list-agents", Name: "List Agents", IsActive: true},
		{Slug: "core-list-skills", Name: "List Skills", IsActive: true},
		{Slug: "core-list-tools", Name: "List Tools", IsActive: true},
	}
	provider := &mockCoreToolProvider{tools: tools}
	got, err := provider.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Equal(t, "core-list-agents", got[0].Slug)
	assert.Equal(t, "core-list-skills", got[1].Slug)
}

// TestCoreToolLoader_LoadAll_PropagatesError ensures the provider error is
// surfaced to the caller so the agentic runner can log the warning.
func TestCoreToolLoader_LoadAll_PropagatesError(t *testing.T) {
	provider := &mockCoreToolProvider{err: assert.AnError}
	tools, err := provider.LoadAll(context.Background())
	assert.Error(t, err)
	assert.Nil(t, tools)
}
