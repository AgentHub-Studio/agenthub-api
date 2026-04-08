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

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// agent + agenthub_manage + ask_user + memory_store (builtins, sorted) + execute-sql (skill)
	require.Len(t, tools, 5)

	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "memory_store", tools[3].Name)
	assert.Equal(t, "execute-sql", tools[4].Name)
	// Description is enriched from the catalog for known slugs.
	assert.Contains(t, tools[4].Description, "Executes SQL queries against configured PostgreSQL datasources")
	assert.Contains(t, string(tools[4].InputSchema), `"query"`)
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
	// agent + agenthub_manage + ask_user + document_search + memory_store (sorted by name)
	require.Len(t, tools, 5)

	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "document_search", tools[3].Name)
	assert.Contains(t, tools[3].Description, "Technical Docs")
	assert.Contains(t, tools[3].Description, "FAQ")
	assert.Contains(t, string(tools[3].InputSchema), `"query"`)
	assert.Equal(t, "memory_store", tools[4].Name)
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
	require.Len(t, tools, 5) // agent + agenthub_manage + ask_user + memory_store (builtins) + http-call (skill)

	// Builtins first, then skill tools.
	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "memory_store", tools[3].Name)
	assert.Equal(t, "http-call", tools[4].Name)
	assert.Contains(t, string(tools[4].InputSchema), `"url"`)
}

func TestToolSchemaBuilder_Build_SkillWithoutSchema_NoToolConfig(t *testing.T) {
	skillID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: skillID, Name: "Empty", Slug: "empty-skill"},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 5) // agent + agenthub_manage + ask_user + memory_store (builtins) + empty-skill (skill)

	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "memory_store", tools[3].Name)
	assert.Equal(t, "empty-skill", tools[4].Name)
	// Should have a default empty schema for the skill.
	assert.Contains(t, string(tools[4].InputSchema), `"type":"object"`)
}

func TestToolSchemaBuilder_Build_NoSkillsNoKBs(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// agent + agenthub_manage + ask_user + memory_store builtins (sorted).
	require.Len(t, tools, 4)
	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "memory_store", tools[3].Name)
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

	builder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 5) // agent + agenthub_manage + ask_user + memory_store (builtins) + bad-schema (skill)
	assert.Equal(t, "agent", tools[0].Name)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.Equal(t, "memory_store", tools[3].Name)
	assert.Equal(t, "bad-schema", tools[4].Name)
	// Should fall back to empty schema for the skill.
	assert.Contains(t, string(tools[4].InputSchema), `"type":"object"`)
}

func TestToolSchemaBuilder_Build_PrefersDatabaseDescriptionOverStaticCatalog(t *testing.T) {
	skills := &mockSkillLister{skills: []skill.Skill{
		{
			ID:          uuid.New(),
			Name:        "Execute SQL",
			Slug:        "execute-sql",
			Description: "Custom DB description for SQL tool.",
			Category:    "data",
		},
	}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 5)
	assert.Equal(t, "execute-sql", tools[4].Name)
	assert.Equal(t, "Custom DB description for SQL tool.", tools[4].Description)
	assert.NotContains(t, tools[4].Description, "PostgreSQL datasources")
}

func TestToolSchemaBuilder_Build_MemoryStoreSchema(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{})
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Len(t, tools, 4) // agent + agenthub_manage + ask_user + memory_store (sorted)

	// memory_store is at index 3 after sorting (agent, agenthub_manage, ask_user, memory_store).
	var schema map[string]any
	require.NoError(t, json.Unmarshal(tools[3].InputSchema, &schema))
	assert.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "content")
	assert.Contains(t, props, "category")
}

func TestToolSchemaBuilder_Build_SortedByPartition(t *testing.T) {
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: uuid.New(), Slug: "zebra-tool", Name: "Zebra", Description: "Z tool"},
		{ID: uuid.New(), Slug: "alpha-tool", Name: "Alpha", Description: "A tool"},
	}}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{{Name: "KB1"}}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// 2 skills + agent + agenthub_manage + ask_user + document_search + memory_store = 7 tools
	require.Len(t, tools, 7)

	// Builtins first (agent, agenthub_manage, ask_user, document_search, memory_store), then skills (alpha-tool, zebra-tool).
	// Within each partition, sorted alphabetically.
	builtinEnd := 0
	for i, t := range tools {
		if !t.Builtin {
			builtinEnd = i
			break
		}
	}
	assert.Equal(t, 5, builtinEnd, "should have 5 builtins as prefix")

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
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: uuid.New(), Slug: "document-search", Name: "Doc Search", Description: "Search docs"},
		{ID: uuid.New(), Slug: "execute-sql", Name: "SQL", Description: "Run SQL"},
	}}
	kbs := &mockKBLister{}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
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
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: uuid.New(), Slug: "zebra-tool", Name: "Zebra", Description: "Z tool"},
		{ID: uuid.New(), Slug: "alpha-tool", Name: "Alpha", Description: "A tool"},
		{ID: uuid.New(), Slug: "mid-tool", Name: "Mid", Description: "M tool"},
	}}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{{Name: "KB1"}}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
	tools, err := builder.Build(context.Background(), uuid.New())

	require.NoError(t, err)
	// 3 skills + agent + agenthub_manage + ask_user + document_search + memory_store = 8 tools
	require.Len(t, tools, 8)

	// Builtins (agent, agenthub_manage, ask_user, document_search, memory_store) should be the first 5, sorted alphabetically.
	assert.True(t, tools[0].Builtin)
	assert.Equal(t, "agent", tools[0].Name)
	assert.True(t, tools[1].Builtin)
	assert.Equal(t, "agenthub_manage", tools[1].Name)
	assert.True(t, tools[2].Builtin)
	assert.Equal(t, "ask_user", tools[2].Name)
	assert.True(t, tools[3].Builtin)
	assert.Equal(t, "document_search", tools[3].Name)
	assert.True(t, tools[4].Builtin)
	assert.Equal(t, "memory_store", tools[4].Name)

	// Skill tools should follow, also sorted alphabetically.
	assert.False(t, tools[5].Builtin)
	assert.Equal(t, "alpha-tool", tools[5].Name)
	assert.False(t, tools[6].Builtin)
	assert.Equal(t, "mid-tool", tools[6].Name)
	assert.False(t, tools[7].Builtin)
	assert.Equal(t, "zebra-tool", tools[7].Name)
}

func TestToolSchemaBuilder_Build_BuiltinFlagIsSet(t *testing.T) {
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: uuid.New(), Slug: "custom-skill", Name: "Custom", Description: "User skill"},
	}}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{{Name: "KB"}}}

	builder := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
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

		// Mark half of them as ShouldDefer via bound tool.
		if i >= 10 {
			toolID := uuid.New()
			toolsMock.bySkill[id] = struct {
				bindings []tool.SkillTool
				tools    []tool.Tool
			}{
				bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: id, ToolID: toolID, IsActive: true}},
				tools:    []tool.Tool{{ID: toolID, Name: slug + "-tool", ShouldDefer: true}},
			}
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
