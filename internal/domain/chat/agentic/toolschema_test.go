package agentic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
)

// --- mock for ToolsBySkillLister ---

type mockToolsBySkill struct {
	bySkill map[uuid.UUID]struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}
}

func newMockToolsBySkill() *mockToolsBySkill {
	return &mockToolsBySkill{
		bySkill: make(map[uuid.UUID]struct {
			bindings []tool.SkillTool
			tools    []tool.Tool
		}),
	}
}

func (m *mockToolsBySkill) ListBySkill(_ context.Context, skillID uuid.UUID) ([]tool.SkillTool, []tool.Tool, error) {
	entry, ok := m.bySkill[skillID]
	if !ok {
		return nil, nil, nil
	}
	return entry.bindings, entry.tools, nil
}

// --- tests ---

func TestToolSchemaBuilder_Build_WithSkills(t *testing.T) {
	skillID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:          skillID,
			Name:        "Execute SQL",
			Slug:        "execute-sql",
			Description: "Run SQL queries against configured datasources",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
	}}
	kbs := &mockKBLister{kbs: nil}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// 1 skill tool + memory_store builtin
	require.Len(t, tools, 2)

	assert.Equal(t, "execute-sql", tools[0].Name)
	// Description is enriched from the catalog for known slugs.
	assert.Contains(t, tools[0].Description, "PostgreSQL datasources")
	assert.Contains(t, string(tools[0].InputSchema), `"query"`)

	assert.Equal(t, "memory_store", tools[1].Name)
}

func TestToolSchemaBuilder_Build_WithKnowledgeBases(t *testing.T) {
	skills := &mockSkillLister{skills: nil}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{
		{Name: "Technical Docs"},
		{Name: "FAQ"},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// document_search + memory_store
	require.Len(t, tools, 2)

	assert.Equal(t, "document_search", tools[0].Name)
	assert.Contains(t, tools[0].Description, "Technical Docs")
	assert.Contains(t, tools[0].Description, "FAQ")
	assert.Contains(t, string(tools[0].InputSchema), `"query"`)

	assert.Equal(t, "memory_store", tools[1].Name)
}

func TestToolSchemaBuilder_Build_SkillWithoutSchema_DerivesFromTool(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()

	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "HTTP Call", Slug: "http-call", Description: "Make HTTP requests"},
	}}

	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:     toolID,
			Name:   "REST Tool",
			Type:   "HTTP",
			Config: json.RawMessage(`{"inputSchema":{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}}`),
		}},
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 2) // http-call + memory_store

	assert.Equal(t, "http-call", tools[0].Name)
	assert.Contains(t, string(tools[0].InputSchema), `"url"`)
}

func TestToolSchemaBuilder_Build_SkillWithoutSchema_NoToolConfig(t *testing.T) {
	skillID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Empty", Slug: "empty-skill"},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 2)

	// Should have a default empty schema.
	assert.Contains(t, string(tools[0].InputSchema), `"type":"object"`)
}

func TestToolSchemaBuilder_Build_NoSkillsNoKBs(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// Only memory_store builtin.
	require.Len(t, tools, 1)
	assert.Equal(t, "memory_store", tools[0].Name)
}

func TestToolSchemaBuilder_Build_InvalidInputSchema(t *testing.T) {
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:          uuid.New(),
			Name:        "Bad Schema",
			Slug:        "bad-schema",
			InputSchema: []byte(`not-json`),
		},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 2)
	// Should fall back to empty schema.
	assert.Contains(t, string(tools[0].InputSchema), `"type":"object"`)
}

func TestToolSchemaBuilder_Build_MemoryStoreSchema(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 1)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(tools[0].InputSchema, &schema))
	assert.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "content")
	assert.Contains(t, props, "category")
}
