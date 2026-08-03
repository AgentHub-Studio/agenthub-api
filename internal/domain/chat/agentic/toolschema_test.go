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

type mockCoreToolProvider struct {
	tools  []agentic.LLMTool
	called bool
}

func (m *mockCoreToolProvider) LoadCoreTools(context.Context) ([]agentic.LLMTool, error) {
	m.called = true
	return m.tools, nil
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
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:   skillID,
			Name: "Execute SQL",
			Slug: "execute-sql",
		},
	}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:     toolID,
			Name:   "SQL Tool",
			Type:   "SQL",
			Config: json.RawMessage(`{"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}`),
		}},
	}
	kbs := &mockKBLister{kbs: nil}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, kbs).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// 9 builtins + execute-sql (skill).
	require.Len(t, tools, 10)

	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "canvas_export_table", tools[3].Name)
	assert.Equal(t, "canvas_feedback", tools[4].Name)
	assert.Equal(t, "canvas_update", tools[5].Name)
	assert.Equal(t, "memory_recall", tools[6].Name)
	assert.Equal(t, "memory_store", tools[7].Name)
	assert.Equal(t, "memory_store_bulk", tools[8].Name)
	assert.Equal(t, "execute-sql", tools[9].Name)
	// Description is enriched from the catalog for known slugs.
	assert.Contains(t, tools[9].Description, "Executes SQL queries against configured PostgreSQL datasources")
	assert.Contains(t, string(tools[9].InputSchema), `"query"`)
}

func TestToolSchemaBuilder_Build_WithKnowledgeBases(t *testing.T) {
	skills := &mockSkillLister{skills: nil}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{
		{Name: "Technical Docs", Status: knowledgebase.StatusActive},
		{Name: "FAQ", Status: knowledgebase.StatusActive},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 10)

	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "canvas_export_table", tools[3].Name)
	assert.Equal(t, "canvas_feedback", tools[4].Name)
	assert.Equal(t, "canvas_update", tools[5].Name)
	assert.Equal(t, "document_search", tools[6].Name)
	assert.Contains(t, tools[6].Description, "Technical Docs")
	assert.Contains(t, tools[6].Description, "FAQ")
	assert.Contains(t, string(tools[6].InputSchema), `"query"`)
	assert.Equal(t, "memory_recall", tools[7].Name)
	assert.Equal(t, "memory_store", tools[8].Name)
	assert.Equal(t, "memory_store_bulk", tools[9].Name)
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

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 10)

	// Builtins first, then skill tools.
	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "canvas_export_table", tools[3].Name)
	assert.Equal(t, "canvas_feedback", tools[4].Name)
	assert.Equal(t, "canvas_update", tools[5].Name)
	assert.Equal(t, "memory_recall", tools[6].Name)
	assert.Equal(t, "memory_store", tools[7].Name)
	assert.Equal(t, "memory_store_bulk", tools[8].Name)
	assert.Equal(t, "http-call", tools[9].Name)
	assert.Contains(t, string(tools[9].InputSchema), `"url"`)
}

func TestToolSchemaBuilder_Build_SkillWithoutSchema_NoToolConfig(t *testing.T) {
	skillID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Empty", Slug: "empty-skill"},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 9)
	for _, tool := range tools {
		assert.NotEqual(t, "empty-skill", tool.Name,
			"skill without active bindings must not appear in tools[]")
	}
}

func TestToolSchemaBuilder_Build_NoSkillsNoKBs(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 9)
	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "canvas_export_table", tools[3].Name)
	assert.Equal(t, "canvas_feedback", tools[4].Name)
	assert.Equal(t, "canvas_update", tools[5].Name)
	assert.Equal(t, "memory_recall", tools[6].Name)
	assert.Equal(t, "memory_store", tools[7].Name)
	assert.Equal(t, "memory_store_bulk", tools[8].Name)
}

func TestToolSchemaBuilder_Build_InvalidInputSchema(t *testing.T) {
	// A skill with a bound tool that has invalid JSON in its inputSchema config.
	skillID := uuid.New()
	toolID := uuid.New()
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:     toolID,
			Name:   "Bad Tool",
			Type:   "HTTP",
			Config: json.RawMessage(`{"inputSchema":"not-a-json-object"}`),
		}},
	}
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:   skillID,
			Name: "Bad Schema",
			Slug: "bad-schema",
		},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 10)
	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "canvas_export_table", tools[3].Name)
	assert.Equal(t, "canvas_feedback", tools[4].Name)
	assert.Equal(t, "canvas_update", tools[5].Name)
	assert.Equal(t, "memory_recall", tools[6].Name)
	assert.Equal(t, "memory_store", tools[7].Name)
	assert.Equal(t, "memory_store_bulk", tools[8].Name)
	assert.Equal(t, "bad-schema", tools[9].Name)
	// Should fall back to empty schema for the skill.
	assert.Contains(t, string(tools[9].InputSchema), `"type":"object"`)
}

// --- TR-01-TASK-20: all active tools exposed, no premature break (P-C175-2) ---

// TestToolSchemaBuilder_Build_SkillWithMultipleTools_ExposesEachToolFlags verifies that
// skills with multiple active tools expose each tool under its own stable slug.
func TestToolSchemaBuilder_Build_SkillWithMultipleTools_ExposesEachToolFlags(t *testing.T) {
	skillID := uuid.New()
	tool1ID := uuid.New()
	tool2ID := uuid.New()

	// Two active tools: first is not destructive, second is destructive.
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{
			{ID: uuid.New(), SkillID: skillID, ToolID: tool1ID, IsActive: true},
			{ID: uuid.New(), SkillID: skillID, ToolID: tool2ID, IsActive: true},
		},
		tools: []tool.Tool{
			{ID: tool1ID, Name: "tool-a", Slug: "tool-a", Type: "HTTP", IsDestructive: false, ReadOnly: true},
			{ID: tool2ID, Name: "tool-b", Slug: "tool-b", Type: "HTTP", IsDestructive: true, ReadOnly: false},
		},
	}
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Multi Tool Skill", Slug: "multi-tool"},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	llmTools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	var destructiveTool *agentic.LLMTool
	for i := range llmTools {
		if llmTools[i].Name == "tool-b" {
			destructiveTool = &llmTools[i]
			break
		}
	}
	require.NotNil(t, destructiveTool, "the destructive bound tool must appear in tools[]")
	assert.True(t, destructiveTool.IsDestructive)
}

// TestToolSchemaBuilder_Build_SkillWithMixedActiveInactive_OnlyActiveCount verifies
// that inactive tools do not contribute flags.
func TestToolSchemaBuilder_Build_SkillWithMixedActiveInactive_OnlyActiveContributes(t *testing.T) {
	skillID := uuid.New()
	tool1ID := uuid.New()
	tool2ID := uuid.New()

	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{
			{ID: uuid.New(), SkillID: skillID, ToolID: tool1ID, IsActive: true},
			{ID: uuid.New(), SkillID: skillID, ToolID: tool2ID, IsActive: false}, // inactive
		},
		tools: []tool.Tool{
			{ID: tool1ID, Name: "active-tool", Type: "HTTP", IsDestructive: false, ReadOnly: true},
			{ID: tool2ID, Name: "inactive-tool", Type: "HTTP", IsDestructive: true, ReadOnly: false},
		},
	}
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Mixed Skill", Slug: "mixed-skill"},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	llmTools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	var mixedTool *agentic.LLMTool
	for i := range llmTools {
		if llmTools[i].Name == "mixed-skill" {
			mixedTool = &llmTools[i]
			break
		}
	}
	require.NotNil(t, mixedTool, "mixed-skill must appear in tools[]")
	// Only the active tool contributes: IsDestructive should remain false.
	assert.False(t, mixedTool.IsDestructive, "inactive tool's IsDestructive must not propagate")
}

func TestToolSchemaBuilder_Build_PrefersDatabaseDescriptionOverStaticCatalog(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:          skillID,
			Name:        "Execute SQL",
			Slug:        "execute-sql",
			Description: "Custom DB description for SQL tool.",
			Category:    "data",
		},
	}}

	// Give the skill an active binding so it appears in tools[] (P-C62-1).
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools:    []tool.Tool{{ID: toolID, Name: "sql-impl", Type: "SQL"}},
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 10)
	assert.Equal(t, "execute-sql", tools[9].Name)
	assert.Equal(t, "Custom DB description for SQL tool.", tools[9].Description)
	assert.NotContains(t, tools[9].Description, "PostgreSQL datasources")
}

func TestToolSchemaBuilder_Build_MemoryStoreSchema(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 9)

	var schema map[string]any
	var found bool
	for _, built := range tools {
		if built.Name != "memory_store" {
			continue
		}
		require.NoError(t, json.Unmarshal(built.InputSchema, &schema))
		found = true
		break
	}
	require.True(t, found, "memory_store builtin must be present")
	assert.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "content")
	assert.Contains(t, props, "category")
}

func TestToolSchemaBuilder_Build_SortedByPartition(t *testing.T) {
	zebraID := uuid.New()
	alphaID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: zebraID, Slug: "zebra-tool", Name: "Zebra", Description: "Z tool"},
		{ID: alphaID, Slug: "alpha-tool", Name: "Alpha", Description: "A tool"},
	}}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{{Name: "KB1", Status: knowledgebase.StatusActive}}}

	// Give each skill an active binding so they appear in tools[] (P-C62-1).
	toolsMock := newMockToolsBySkill()
	for _, id := range []uuid.UUID{zebraID, alphaID} {
		toolID := uuid.New()
		toolsMock.bySkill[id] = struct {
			bindings []tool.SkillTool
			tools    []tool.Tool
		}{
			bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: id, ToolID: toolID, IsActive: true}},
			tools:    []tool.Tool{{ID: toolID, Name: "impl"}},
		}
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, kbs).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 12)

	// Builtins first (10 builtins), then skills (alpha-tool, zebra-tool).
	// Within each partition, sorted alphabetically.
	builtinEnd := 0
	for i, t := range tools {
		if !t.Builtin {
			builtinEnd = i
			break
		}
	}
	assert.Equal(t, 10, builtinEnd, "should have 10 builtins as prefix")

	// Builtins sorted.
	for i := 1; i < builtinEnd; i++ {
		assert.True(t, tools[i-1].Name <= tools[i].Name,
			"builtins should be sorted: %s <= %s", tools[i-1].Name, tools[i].Name)
	}
	// Skills sorted.
	for i := builtinEnd + 1; i < len(tools); i++ {
		assert.True(t, tools[i-1].Name <= tools[i].Name,
			"skills should be sorted: %s <= %s", tools[i-1].Name, tools[i].Name)
	}
}

func TestToolSchemaBuilder_ReadOnlyFlag(t *testing.T) {
	docID := uuid.New()
	sqlID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: docID, Slug: "document-search", Name: "Doc Search", Description: "Search docs"},
		{ID: sqlID, Slug: "execute-sql", Name: "SQL", Description: "Run SQL"},
	}}
	kbs := &mockKBLister{}

	// Give each skill an active binding so it appears in tools[] (P-C62-1).
	toolsMock := newMockToolsBySkill()
	for _, id := range []uuid.UUID{docID, sqlID} {
		toolID := uuid.New()
		toolsMock.bySkill[id] = struct {
			bindings []tool.SkillTool
			tools    []tool.Tool
		}{
			bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: id, ToolID: toolID, IsActive: true}},
			tools:    []tool.Tool{{ID: toolID, Name: "impl"}},
		}
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)

	// Find tools by name and check ReadOnly flag.
	toolMap := map[string]agentic.LLMTool{}
	for _, t := range tools {
		toolMap[t.Name] = t
	}

	assert.True(t, toolMap["document-search"].ReadOnly, "document-search should be read-only")
	assert.False(t, toolMap["execute-sql"].ReadOnly, "execute-sql should NOT be read-only")
	assert.False(t, toolMap["memory_store"].ReadOnly, "memory_store should NOT be read-only")
}

func TestToolSchemaBuilder_Build_BuiltinsFormContiguousPrefix(t *testing.T) {
	zebraID := uuid.New()
	alphaID := uuid.New()
	midID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: zebraID, Slug: "zebra-tool", Name: "Zebra", Description: "Z tool"},
		{ID: alphaID, Slug: "alpha-tool", Name: "Alpha", Description: "A tool"},
		{ID: midID, Slug: "mid-tool", Name: "Mid", Description: "M tool"},
	}}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{{Name: "KB1", Status: knowledgebase.StatusActive}}}

	// Give each skill an active binding so it appears in tools[] (P-C62-1).
	toolsMock := newMockToolsBySkill()
	for _, id := range []uuid.UUID{zebraID, alphaID, midID} {
		toolID := uuid.New()
		toolsMock.bySkill[id] = struct {
			bindings []tool.SkillTool
			tools    []tool.Tool
		}{
			bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: id, ToolID: toolID, IsActive: true}},
			tools:    []tool.Tool{{ID: toolID, Name: "impl"}},
		}
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, kbs).
		WithAdminScope(true).WithEnableManagement(true)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 13)

	// Builtins (10 total) should be the first 10, sorted alphabetically.
	assert.True(t, tools[0].Builtin)
	assert.Equal(t, "agent", tools[0].Name)
	assert.True(t, tools[1].Builtin)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.True(t, tools[2].Builtin)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.True(t, tools[3].Builtin)
	assert.Equal(t, "canvas_export_table", tools[3].Name)
	assert.True(t, tools[4].Builtin)
	assert.Equal(t, "canvas_feedback", tools[4].Name)
	assert.True(t, tools[5].Builtin)
	assert.Equal(t, "canvas_update", tools[5].Name)
	assert.True(t, tools[6].Builtin)
	assert.Equal(t, "document_search", tools[6].Name)
	assert.True(t, tools[7].Builtin)
	assert.Equal(t, "memory_recall", tools[7].Name)
	assert.True(t, tools[8].Builtin)
	assert.Equal(t, "memory_store", tools[8].Name)
	assert.True(t, tools[9].Builtin)
	assert.Equal(t, "memory_store_bulk", tools[9].Name)

	// Skill tools should follow, also sorted alphabetically.
	assert.False(t, tools[10].Builtin)
	assert.Equal(t, "alpha-tool", tools[10].Name)
	assert.False(t, tools[11].Builtin)
	assert.Equal(t, "mid-tool", tools[11].Name)
	assert.False(t, tools[12].Builtin)
	assert.Equal(t, "zebra-tool", tools[12].Name)
}

func TestToolSchemaBuilder_Build_BuiltinFlagIsSet(t *testing.T) {
	customID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: customID, Slug: "custom-skill", Name: "Custom", Description: "User skill"},
	}}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{{Name: "KB", Status: knowledgebase.StatusActive}}}

	// Give the skill an active binding so it appears in tools[] (P-C62-1).
	toolsMock := newMockToolsBySkill()
	toolID := uuid.New()
	toolsMock.bySkill[customID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: customID, ToolID: toolID, IsActive: true}},
		tools:    []tool.Tool{{ID: toolID, Name: "impl"}},
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)

	toolMap := map[string]agentic.LLMTool{}
	for _, t := range tools {
		toolMap[t.Name] = t
	}

	assert.True(t, toolMap["agent"].Builtin, "agent should be builtin")
	assert.True(t, toolMap["document_search"].Builtin, "document_search should be builtin")
	assert.True(t, toolMap["memory_store"].Builtin, "memory_store should be builtin")
	assert.False(t, toolMap["custom-skill"].Builtin, "custom-skill should NOT be builtin")
}

func TestIsReadOnlyTool(t *testing.T) {
	assert.True(t, agentic.IsReadOnlyTool("document-search"))
	assert.True(t, agentic.IsReadOnlyTool("document_search"))
	assert.True(t, agentic.IsReadOnlyTool("web-scraper"))
	assert.False(t, agentic.IsReadOnlyTool("execute-sql"))
	assert.False(t, agentic.IsReadOnlyTool("send-email"))
	assert.False(t, agentic.IsReadOnlyTool("memory_store"))
	assert.False(t, agentic.IsReadOnlyTool("unknown-tool"))
}

func TestToolSchemaBuilder_BuildWithDeferred_BelowThreshold(t *testing.T) {
	// With fewer tools than DeferredToolThreshold, all should be loaded (no deferred).
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: uuid.New(), Slug: "execute-sql", Name: "SQL"},
	}}
	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), &mockKBLister{})
	result, err := builder.BuildWithDeferred(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Len(t, result.Loaded, len(result.All), "below threshold: all tools should be loaded")
	assert.Nil(t, result.Deferred, "below threshold: no deferred tools")
}

func TestToolSchemaBuilder_BuildWithDeferred_AboveThreshold(t *testing.T) {
	// Create enough skills to exceed DeferredToolThreshold (15).
	var skills []skill.Skill
	toolsMock := newMockToolsBySkill()
	for i := 0; i < 20; i++ {
		id := uuid.New()
		slug := "skill-" + string(rune('a'+i))
		sk := skill.Skill{ID: id, Name: slug, Slug: slug}
		skills = append(skills, sk)

		toolID := uuid.New()
		// All skills get active bindings (P-C62-1: skills without bindings are excluded).
		// Mark the second half as ShouldDefer via bound tool.
		toolsMock.bySkill[id] = struct {
			bindings []tool.SkillTool
			tools    []tool.Tool
		}{
			bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: id, ToolID: toolID, IsActive: true}},
			tools:    []tool.Tool{{ID: toolID, Name: slug + "-tool", ShouldDefer: i >= 10}},
		}
	}

	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{skills: skills}, toolsMock, &mockKBLister{})
	result, err := builder.BuildWithDeferred(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Greater(t, len(result.All), agentic.DeferredToolThreshold,
		"total tools should exceed threshold")
	assert.Greater(t, len(result.Deferred), 0, "should have deferred tools")
	assert.Less(t, len(result.Loaded), len(result.All),
		"loaded should be fewer than all")

	// Deferred tool names should not appear in loaded.
	loadedNames := map[string]bool{}
	for _, t := range result.Loaded {
		loadedNames[t.Name] = true
	}
	for _, tool := range result.Deferred {
		assert.False(t, loadedNames[tool.Name],
			"deferred tool %s should not be in loaded set", tool.Name)
	}

	// tool_search builtin should be in loaded.
	assert.True(t, loadedNames["tool_search"], "tool_search should be in loaded when deferred exist")
}

func TestToolBuildResult_DeferredToolNames(t *testing.T) {
	result := &agentic.ToolBuildResult{
		Deferred: []agentic.LLMTool{
			{Name: "tool-a"},
			{Name: "tool-b"},
			{Name: "tool-c"},
		},
	}
	names := result.DeferredToolNames()
	assert.Equal(t, []string{"tool-a", "tool-b", "tool-c"}, names)
}

func TestToolBuildResult_DeferredToolNames_Empty(t *testing.T) {
	result := &agentic.ToolBuildResult{}
	names := result.DeferredToolNames()
	assert.Empty(t, names)
}

// --- TR-01-TASK-01: enable_management gate tests (P-C184-2, P-C281-1) ---

func toolNames(tools []agentic.LLMTool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

// TestBuildTools_ManagementExcludedWhenFlagFalse verifies that agenthub_manage is
// absent when enable_management=false, even when adminScope=true.
func TestBuildTools_ManagementExcludedWhenFlagFalse(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithAdminScope(true).
		WithEnableManagement(false) // explicit opt-out

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	for _, tool := range tools {
		assert.NotEqual(t, "agenthub_manage", tool.Name,
			"agenthub_manage must not appear when enable_management=false")
	}
}

// TestBuildTools_ManagementIncludedWhenFlagTrue verifies that agenthub_manage is
// present when both enable_management=true and adminScope=true at depth 0.
func TestBuildTools_ManagementIncludedWhenFlagTrue(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithAdminScope(true).
		WithEnableManagement(true)

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	assert.Contains(t, toolNames(tools), "agenthub_manage",
		"agenthub_manage must appear when enable_management=true and adminScope=true at depth 0")
}

// TestBuildTools_SubAgentDepthGTZero_NeverHasManagement verifies that agenthub_manage
// is excluded from sub-agents even when both flags are true (P-C281-1).
func TestBuildTools_SubAgentDepthGTZero_NeverHasManagement(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithAdminScope(true).
		WithEnableManagement(true).
		WithDepthLimits(1, 3) // depth=1 = sub-agent

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	for _, tool := range tools {
		assert.NotEqual(t, "agenthub_manage", tool.Name,
			"agenthub_manage must never appear in sub-agent toolset (depth > 0)")
	}
}

// TestBuildTools_ManagementExcludedWhenNoAdminScope verifies that agenthub_manage
// is absent when enable_management=true but adminScope=false (non-admin caller).
func TestBuildTools_ManagementExcludedWhenNoAdminScope(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithAdminScope(false). // non-admin caller
		WithEnableManagement(true)

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	for _, tool := range tools {
		assert.NotEqual(t, "agenthub_manage", tool.Name,
			"agenthub_manage must not appear for non-admin callers even when enable_management=true")
	}
}

func TestBuildTools_CoreToolsExcludedWithoutManagementScope(t *testing.T) {
	coreTools := &mockCoreToolProvider{
		tools: []agentic.LLMTool{{Name: "core-list-agents", Description: "List agents"}},
	}
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithCoreToolProvider(coreTools).
		WithAdminScope(true).
		WithEnableManagement(false)

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	assert.False(t, coreTools.called, "core tools must not be loaded outside management scope")
	assert.NotContains(t, toolNames(tools), "core-list-agents")
}

func TestBuildTools_CoreToolsIncludedWithManagementScope(t *testing.T) {
	coreTools := &mockCoreToolProvider{
		tools: []agentic.LLMTool{{Name: "core-list-agents", Description: "List agents"}},
	}
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{}).
		WithCoreToolProvider(coreTools).
		WithAdminScope(true).
		WithEnableManagement(true)

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	assert.True(t, coreTools.called, "core tools should load only inside management scope")
	assert.Contains(t, toolNames(tools), "core-list-agents")
}

// --- TR-01-TASK-06: orphaned skill instructions (P-C152-2, P-C159-1, P-C168-1) ---

// TestBuildTools_InstructionOnlySkill_NotInToolArray verifies that a skill with
// instructions but no active tool bindings is excluded from the LLM tools[] array.
func TestBuildTools_InstructionOnlySkill_NotInToolArray(t *testing.T) {
	skillID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:           skillID,
			Name:         "Behavior Skill",
			Slug:         "behavior-skill",
			Instructions: "always call check_status tool",
		},
	}}

	// Empty toolsBySkill = no active bindings → skill should be excluded.
	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	for _, tool := range tools {
		assert.NotEqual(t, "behavior-skill", tool.Name,
			"instruction-only skill with no active bindings must not appear in tools[]")
	}
}

// TestBuildTools_SkillWithTools_InToolArray verifies that a skill with an active
// tool binding does appear in the LLM tools[] array.
func TestBuildTools_SkillWithTools_InToolArray(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Search Skill", Slug: "search-skill"},
	}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools:    []tool.Tool{{ID: toolID, Name: "web_search"}},
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Contains(t, toolNames(tools), "search-skill",
		"skill with active binding must appear in tools[]")
}

func TestBuildTools_MCPDuplicateOfNativeSkill_IsSkipped(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:   skillID,
			Name: "ViaCEP Address Lookup",
			Slug: "viacep_address_lookup",
		},
	}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools:    []tool.Tool{{ID: toolID, Name: "ViaCEP Tool"}},
	}

	mcpClient := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "agenthub-platform-mcp", Name: "viacep_address_lookup"},
			{ServerName: "agenthub-platform-mcp", Name: "github_repository_lookup"},
		},
	}
	mcpBridge := agentic.NewMCPToolBridge(mcpClient, "test")
	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}).
		WithMCPBridge(mcpBridge)

	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	names := toolNames(tools)
	assert.Contains(t, names, "viacep_address_lookup")
	assert.NotContains(t, names, "mcp__agenthub-platform-mcp__viacep_address_lookup")
	assert.Contains(t, names, "mcp__agenthub-platform-mcp__github_repository_lookup")
}

// --- TR-01-TASK-07: FilterActiveKBs (P-C179-1, P-C168-1) ---

// TestFilterActiveKBs_ExcludesPaused verifies that PAUSED KBs are excluded.
func TestFilterActiveKBs_ExcludesPaused(t *testing.T) {
	kbs := []knowledgebase.KnowledgeBase{
		{Name: "Active KB", Status: knowledgebase.StatusActive},
		{Name: "Paused KB", Status: knowledgebase.StatusPaused},
	}

	active := agentic.FilterActiveKBs(kbs)

	require.Len(t, active, 1)
	assert.Equal(t, "Active KB", active[0].Name)
}

// TestFilterActiveKBs_AllActive verifies that all KBs are returned when all are active.
func TestFilterActiveKBs_AllActive(t *testing.T) {
	kbs := []knowledgebase.KnowledgeBase{
		{Name: "KB1", Status: knowledgebase.StatusActive},
		{Name: "KB2", Status: knowledgebase.StatusActive},
	}

	active := agentic.FilterActiveKBs(kbs)

	assert.Len(t, active, 2)
}

// TestFilterActiveKBs_AllPaused_ReturnsNil verifies that nil is returned when all KBs
// are paused (prevents document_search from being offered to the LLM).
func TestFilterActiveKBs_AllPaused_ReturnsNil(t *testing.T) {
	kbs := []knowledgebase.KnowledgeBase{
		{Name: "Paused KB", Status: knowledgebase.StatusPaused},
	}

	active := agentic.FilterActiveKBs(kbs)

	assert.Nil(t, active)
}

// TestBuildTools_PausedKB_NoDocumentSearch verifies that document_search is NOT added
// to the tools[] when all KBs are PAUSED.
func TestBuildTools_PausedKB_NoDocumentSearch(t *testing.T) {
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{
		{Name: "Paused KB", Status: knowledgebase.StatusPaused},
	}}

	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	for _, tool := range tools {
		assert.NotEqual(t, "document_search", tool.Name, "document_search must not appear when all KBs are PAUSED")
	}
}

// TestBuildTools_ActiveKB_DocumentSearchPresent verifies that document_search IS added
// when at least one KB is ACTIVE.
func TestBuildTools_ActiveKB_DocumentSearchPresent(t *testing.T) {
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{
		{Name: "Active KB", Status: knowledgebase.StatusActive},
	}}

	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Contains(t, toolNames(tools), "document_search")
}

// --- TR-01-TASK-10: InputSchema preference over derived schema (P-C175-1/P-C175-2) ---

// TestBuildTools_ExplicitInputSchema_UsedOverDerived verifies that when a bound tool
// has an explicit InputSchema, it is used verbatim instead of the auto-derived schema.
func TestBuildTools_ExplicitInputSchema_UsedOverDerived(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()
	explicit := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","enum":["a","b","c"]}},"required":["query"]}`)

	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Search", Slug: "search-skill"},
	}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:          toolID,
			Name:        "search_http",
			InputSchema: explicit,
			Config:      []byte(`{"url":"https://example.com/{query}","method":"GET"}`),
		}},
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	idx := -1
	for i, t := range tools {
		if t.Name == "search-skill" {
			idx = i
			break
		}
	}
	require.GreaterOrEqual(t, idx, 0, "search-skill must be in tools")
	// Explicit schema should contain the enum — derived schema would not.
	assert.Contains(t, string(tools[idx].InputSchema), `"enum"`, "explicit InputSchema must be used")
}

// TestBuildTools_NoInputSchema_DerivedFromConfig verifies that when InputSchema is
// nil, the schema is derived from the tool config URL template.
func TestBuildTools_NoInputSchema_DerivedFromConfig(t *testing.T) {
	skillID := uuid.New()
	toolID := uuid.New()

	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Fetch", Slug: "fetch-skill"},
	}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:     toolID,
			Name:   "fetch_http",
			Config: []byte(`{"url":"https://example.com/{city}","method":"GET"}`),
		}},
	}

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	idx := -1
	for i, t := range tools {
		if t.Name == "fetch-skill" {
			idx = i
			break
		}
	}
	require.GreaterOrEqual(t, idx, 0, "fetch-skill must be in tools")
	// Derived schema should contain "city" from URL template.
	assert.Contains(t, string(tools[idx].InputSchema), `"city"`, "schema must be derived from URL template")
}

// TestMemoryStoreTool_DescriptionContainsDisclaimer verifies the TTL/hallucination
// disclaimer is present in the memory_store tool description (ACT-F3-17 / P-C339-1).
func TestMemoryStoreTool_DescriptionContainsDisclaimer(t *testing.T) {
	tool := agentic.ExportedMemoryStoreTool()
	assert.Contains(t, tool.Description, "IMPORTANT")
	assert.Contains(t, tool.Description, "expire")
	assert.Contains(t, tool.Description, "fabricate")
}
