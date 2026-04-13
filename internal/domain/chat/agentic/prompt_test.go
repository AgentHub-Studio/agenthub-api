package agentic_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// --- mocks ---

type mockSkillLister struct {
	skills []skill.Skill
}

func (m *mockSkillLister) ListByAgentID(_ context.Context, _ uuid.UUID) ([]skill.Skill, error) {
	return m.skills, nil
}

func (m *mockSkillLister) ListByIDs(_ context.Context, ids []uuid.UUID) ([]skill.Skill, error) {
	// Filter m.skills to only those whose ID appears in ids.
	var result []skill.Skill
	for _, s := range m.skills {
		for _, id := range ids {
			if s.ID == id {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

func (m *mockSkillLister) List(_ context.Context, _ *string, _ pagination.PageRequest) ([]skill.Skill, int64, error) {
	return nil, 0, nil
}

type mockKBLister struct {
	kbs []knowledgebase.KnowledgeBase
}

func (m *mockKBLister) ListByAgentID(_ context.Context, _ uuid.UUID) ([]knowledgebase.KnowledgeBase, error) {
	return m.kbs, nil
}

func (m *mockKBLister) List(_ context.Context, _ pagination.PageRequest) ([]knowledgebase.KnowledgeBase, int64, error) {
	return nil, 0, nil
}

type mockSummaryFinder struct {
	msg   chat.ChatMessage
	found bool
}

func (m *mockSummaryFinder) GetLatestCompactSummary(_ context.Context, _ uuid.UUID) (chat.ChatMessage, bool, error) {
	return m.msg, m.found, nil
}

type mockPromptTemplateResolver struct {
	templates map[string]string
}

func (m *mockPromptTemplateResolver) ResolvePromptTemplate(_ context.Context, _ uuid.UUID, slug string) (string, bool, error) {
	content, ok := m.templates[slug]
	return content, ok, nil
}

// --- tests ---

func TestPromptBuilder_Build_AllSections(t *testing.T) {
	skills := &mockSkillLister{skills: []skill.Skill{
		{Name: "Document Search", Slug: "document-search", Description: "Search documents in the knowledge base"},
		{Name: "Execute SQL", Slug: "execute-sql", Description: "Execute SQL queries"},
	}}
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{
		{Name: "Technical Docs", DocumentCount: 245, Description: "Internal technical documentation"},
	}}
	summary := &mockSummaryFinder{
		msg:   chat.ChatMessage{Content: "User asked about API endpoints and received a list of 5 endpoints."},
		found: true,
	}

	builder := agentic.NewPromptBuilder(skills, kbs, summary, agentic.DefaultPromptConfig())

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:      uuid.New(),
		SessionID:    uuid.New(),
		SystemPrompt: "You are a helpful assistant for financial analysis.",
		Memories:     "[2026-03-15] User prefers answers in Portuguese.",
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "You are a helpful assistant for financial analysis.")
	assert.Contains(t, prompt, "## Available Tools")
	assert.Contains(t, prompt, "document-search")
	assert.Contains(t, prompt, "execute-sql")
	assert.Contains(t, prompt, "## Tool Usage Instructions")
	assert.Contains(t, prompt, "## Knowledge Bases")
	assert.Contains(t, prompt, "Technical Docs")
	assert.Contains(t, prompt, "245 documents")
	assert.Contains(t, prompt, "## Relevant Memories")
	assert.Contains(t, prompt, "Portuguese")
	assert.Contains(t, prompt, "## Conversation Summary")
	assert.Contains(t, prompt, "API endpoints")
}

func TestPromptBuilder_Build_MinimalPrompt(t *testing.T) {
	skills := &mockSkillLister{skills: nil}
	kbs := &mockKBLister{kbs: nil}
	summary := &mockSummaryFinder{found: false}

	builder := agentic.NewPromptBuilder(skills, kbs, summary, agentic.DefaultPromptConfig())

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:      uuid.New(),
		SessionID:    uuid.New(),
		SystemPrompt: "You are an assistant.",
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "You are an assistant.")
	assert.Contains(t, prompt, "## Tool Usage Instructions")
	assert.NotContains(t, prompt, "## Available Tools")
	assert.NotContains(t, prompt, "## Knowledge Bases")
	assert.NotContains(t, prompt, "## Relevant Memories")
	assert.NotContains(t, prompt, "## Conversation Summary")
}

func TestPromptBuilder_Build_NoSystemPrompt(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
	})

	require.NoError(t, err)
	// Should still have tool usage instructions at minimum.
	assert.Contains(t, prompt, "## Tool Usage Instructions")
}

func TestPromptBuilder_Build_Truncation(t *testing.T) {
	// Set a very small token budget to trigger truncation.
	cfg := agentic.PromptConfig{MaxEstimatedTokens: 10} // 40 chars

	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		cfg,
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:      uuid.New(),
		SessionID:    uuid.New(),
		SystemPrompt: strings.Repeat("A", 200),
	})

	require.NoError(t, err)
	assert.LessOrEqual(t, len(prompt), 40)
}

func TestPromptBuilder_Build_NilDependencies(t *testing.T) {
	// All nil — should not panic.
	builder := agentic.NewPromptBuilder(nil, nil, nil, agentic.DefaultPromptConfig())

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:      uuid.New(),
		SessionID:    uuid.New(),
		SystemPrompt: "Identity only.",
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "Identity only.")
}

func TestPromptBuilder_Build_KBWithoutDescription(t *testing.T) {
	kbs := &mockKBLister{kbs: []knowledgebase.KnowledgeBase{
		{Name: "FAQ", DocumentCount: 10},
	}}
	builder := agentic.NewPromptBuilder(&mockSkillLister{}, kbs, &mockSummaryFinder{found: false}, agentic.DefaultPromptConfig())

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "FAQ")
	assert.Contains(t, prompt, "10 documents")
}

func TestPromptBuilder_Build_SkillWithoutDescription(t *testing.T) {
	skills := &mockSkillLister{skills: []skill.Skill{
		{Name: "My Tool", Slug: "my-tool"},
	}}
	builder := agentic.NewPromptBuilder(skills, &mockKBLister{}, &mockSummaryFinder{found: false}, agentic.DefaultPromptConfig())

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "**My Tool** (`my-tool`)")
}

func TestPromptBuilder_Build_DeferredTools(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:           uuid.New(),
		SessionID:         uuid.New(),
		SystemPrompt:      "You are an assistant.",
		DeferredToolNames: []string{"agenthub_create_agent", "agenthub_delete_skill"},
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "## Deferred Tools")
	assert.Contains(t, prompt, "`agenthub_create_agent`")
	assert.Contains(t, prompt, "`agenthub_delete_skill`")
	assert.Contains(t, prompt, "tool_search")
}

func TestPromptBuilder_Build_UserOnlySkills(t *testing.T) {
	// Skills with DisableModelInvocation=true should appear in User Commands,
	// not in Available Tools.
	skills := &mockSkillLister{skills: []skill.Skill{
		{Name: "Document Search", Slug: "document-search", Description: "Search docs"},
		{Name: "Debug Agent", Slug: "debug-agent", Description: "Diagnose agent failures", DisableModelInvocation: true},
		{Name: "Optimize Agent", Slug: "optimize-agent", Description: "Optimize agent config", DisableModelInvocation: true},
	}}
	builder := agentic.NewPromptBuilder(skills, &mockKBLister{}, &mockSummaryFinder{found: false}, agentic.DefaultPromptConfig())

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
		UserOnlySkills: []skill.Skill{
			{Name: "Debug Agent", Slug: "debug-agent", Description: "Diagnose agent failures"},
			{Name: "Optimize Agent", Slug: "optimize-agent", Description: "Optimize agent config"},
		},
	})

	require.NoError(t, err)
	// User-only skills should be in User Commands section.
	assert.Contains(t, prompt, "## User Commands")
	assert.Contains(t, prompt, "/debug-agent")
	assert.Contains(t, prompt, "/optimize-agent")
	// User-only skills should NOT be in Available Tools section.
	assert.Contains(t, prompt, "## Available Tools")
	assert.Contains(t, prompt, "document-search")
	// debug-agent should not appear in Available Tools (DisableModelInvocation=true).
	toolsIdx := strings.Index(prompt, "## Available Tools")
	userCmdIdx := strings.Index(prompt, "## User Commands")
	if toolsIdx >= 0 && userCmdIdx >= 0 {
		toolsSection := prompt[toolsIdx:userCmdIdx]
		assert.NotContains(t, toolsSection, "debug-agent")
		assert.NotContains(t, toolsSection, "optimize-agent")
	}
}

func TestPromptBuilder_Build_NoDeferredTools(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:      uuid.New(),
		SessionID:    uuid.New(),
		SystemPrompt: "You are an assistant.",
	})

	require.NoError(t, err)
	assert.NotContains(t, prompt, "## Deferred Tools")
}

func TestPromptBuilder_Build_UsesPromptTemplateOverrides(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	).WithPromptTemplateResolver(&mockPromptTemplateResolver{
		templates: map[string]string{
			"agentic-user-interaction-policy": "## Custom Input Policy\nAlways collect structured input.",
			"agentic-tool-usage-instructions": "## Custom Tool Rules\nAlways explain tool choices.",
		},
	})

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:      uuid.New(),
		SessionID:    uuid.New(),
		SystemPrompt: "You are an assistant.",
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "## Custom Input Policy")
	assert.Contains(t, prompt, "## Custom Tool Rules")
	assert.NotContains(t, prompt, "## CRITICAL — User Input Policy")
	assert.NotContains(t, prompt, "## Tool Usage Instructions")
}

func TestPromptBuilder_Build_FallsBackWhenPromptTemplateMissing(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	).WithPromptTemplateResolver(&mockPromptTemplateResolver{
		templates: map[string]string{},
	})

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "## CRITICAL — User Input Policy")
	assert.Contains(t, prompt, "## Tool Usage Instructions")
}

// --- TR-01-TASK-06: FormatSkillInstructionsSection (P-C152-2, P-C159-1, P-C168-1) ---

// TestFormatSkillInstructions_SkillWithNoActiveTools_OmittedFromPrompt verifies that
// a skill with no active tools whose instructions reference a tool name is omitted.
func TestFormatSkillInstructions_SkillWithNoActiveTools_OmittedFromPrompt(t *testing.T) {
	sw := agentic.SkillWithTools{
		Skill:       skill.Skill{Instructions: "always call check_status tool"},
		ActiveTools: []tool.Tool{}, // no active tools
	}
	result := agentic.FormatSkillInstructionsSection([]agentic.SkillWithTools{sw})
	assert.Empty(t, result)
}

// TestFormatSkillInstructions_SkillWithActiveTools_Included verifies that a skill
// with active tools always has its instructions included.
func TestFormatSkillInstructions_SkillWithActiveTools_Included(t *testing.T) {
	sw := agentic.SkillWithTools{
		Skill:       skill.Skill{Instructions: "use this skill to answer questions"},
		ActiveTools: []tool.Tool{{Name: "search"}},
	}
	result := agentic.FormatSkillInstructionsSection([]agentic.SkillWithTools{sw})
	assert.Contains(t, result, "use this skill")
}

// TestFormatSkillInstructions_InstructionOnlySkillWithoutToolRef_Included verifies
// that behavioral instructions (no tool reference) survive even without active tools.
func TestFormatSkillInstructions_InstructionOnlySkillWithoutToolRef_Included(t *testing.T) {
	sw := agentic.SkillWithTools{
		Skill:       skill.Skill{Instructions: "always respond in Portuguese"},
		ActiveTools: []tool.Tool{},
	}
	result := agentic.FormatSkillInstructionsSection([]agentic.SkillWithTools{sw})
	assert.Contains(t, result, "Portuguese")
}

// --- TR-01-TASK-18: Anti-hallucination guard (P-C127-3, P-C130-1, P-C142-2) ---

// TestPrompt_ContainsAntiHallucinationGuard verifies that the guard is always present.
func TestPrompt_ContainsAntiHallucinationGuard(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:      uuid.New(),
		SessionID:    uuid.New(),
		SystemPrompt: "You are a helpful assistant.",
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "Tool Usage Rules")
	assert.Contains(t, prompt, "do not fabricate")
	assert.Contains(t, prompt, "I don't have access to that capability")
}

// TestPrompt_AgentWithoutSystemPrompt_HasDefaultGuard verifies the guard is injected
// even when the agent has no systemPrompt set.
func TestPrompt_AgentWithoutSystemPrompt_HasDefaultGuard(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
		// No SystemPrompt
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "Tool Usage Rules")
	assert.NotEmpty(t, prompt)
}

// TestPromptBuild_BehavioralSkillInstructions_IncludedInPrompt verifies that skill
// instructions without tool references are injected into the system prompt by Build.
// This is the production-path integration of FormatSkillInstructionsSection.
// P-C152-2 / B6 regression: instructions were being discarded before this fix.
func TestPromptBuild_BehavioralSkillInstructions_IncludedInPrompt(t *testing.T) {
	marker := "---END_MARKER---"
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{skills: []skill.Skill{
			{Name: "Formatter", Slug: "formatter", Instructions: "Always end EVERY response with " + marker},
		}},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, marker, "behavioral skill instruction must appear in the system prompt")
}

// TestPromptBuild_ToolRefSkillInstructions_OmittedFromPrompt verifies that instructions
// containing a tool reference are omitted when we cannot verify active tool bindings.
// This prevents LLM hallucination of non-existent tool calls. P-C152-2.
func TestPromptBuild_ToolRefSkillInstructions_OmittedFromPrompt(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{skills: []skill.Skill{
			{Name: "Search", Slug: "search", Instructions: "Use the document_search tool to find answers."},
		}},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
	})

	require.NoError(t, err)
	assert.NotContains(t, prompt, "document_search tool", "tool-referencing instruction must be omitted to prevent hallucination")
}

// TestPromptBuilder_ClearCacheForAgent_InvalidatesSkillInstructions verifies that
// ClearCacheForAgent causes fresh skill data to be fetched on the next Build call.
// P-C343-1: cache must not serve stale skill instructions after an edit.
func TestPromptBuilder_ClearCacheForAgent_InvalidatesSkillInstructions(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()

	// First build: skill says "respond in Portuguese"
	lister := &mockSkillLister{skills: []skill.Skill{
		{Name: "Language", Slug: "language", Instructions: "Always respond in Portuguese."},
	}}
	builder := agentic.NewPromptBuilder(lister, &mockKBLister{}, &mockSummaryFinder{}, agentic.DefaultPromptConfig())

	prompt1, err := builder.Build(context.Background(), agentic.PromptInput{AgentID: agentID, SessionID: sessionID})
	require.NoError(t, err)
	assert.Contains(t, prompt1, "Portuguese")

	// Simulate skill edit: now says "respond in Spanish"
	lister.skills = []skill.Skill{
		{Name: "Language", Slug: "language", Instructions: "Always respond in Spanish."},
	}

	// Without clearing cache: stale result
	promptStale, err := builder.Build(context.Background(), agentic.PromptInput{AgentID: agentID, SessionID: sessionID})
	require.NoError(t, err)
	assert.Contains(t, promptStale, "Portuguese", "before cache clear: stale result expected")
	assert.NotContains(t, promptStale, "Spanish")

	// After clearing cache for this agent: fresh result
	builder.ClearCacheForAgent(agentID)
	promptFresh, err := builder.Build(context.Background(), agentic.PromptInput{AgentID: agentID, SessionID: sessionID})
	require.NoError(t, err)
	assert.Contains(t, promptFresh, "Spanish", "after cache clear: fresh result expected")
	assert.NotContains(t, promptFresh, "Portuguese")
}

// TestPromptBuild_ActiveSkillSlugs_ExcludesEmptyToolSkill verifies that a skill
// without active tool bindings is omitted from "## Available Tools" when the
// caller provides a non-empty ActiveSkillSlugs set. BUG-SKILL-EMPTY: the LLM
// was hallucinating calls to slugs that appeared in the system prompt even though
// P-C62-1 already excluded them from the JSON tools[] array.
func TestPromptBuild_ActiveSkillSlugs_ExcludesEmptyToolSkill(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()

	lister := &mockSkillLister{skills: []skill.Skill{
		{Name: "Active Tool", Slug: "active-tool", Description: "Has bound tools"},
		{Name: "Empty Tool", Slug: "empty-tool", Description: "No bound tools"},
	}}
	builder := agentic.NewPromptBuilder(lister, &mockKBLister{}, &mockSummaryFinder{}, agentic.DefaultPromptConfig())

	// Only "active-tool" has an active binding.
	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:          agentID,
		SessionID:        sessionID,
		ActiveSkillSlugs: map[string]bool{"active-tool": true},
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "active-tool", "skill with active binding must appear in Available Tools")
	assert.NotContains(t, prompt, "empty-tool", "skill without active binding must be omitted from Available Tools")
}

// TestPromptBuild_ActiveSkillSlugs_EmptySetShowsAll verifies that when
// ActiveSkillSlugs is nil/empty, all skills are listed (backward-compatible
// behaviour for callers that do not supply tool binding info).
func TestPromptBuild_ActiveSkillSlugs_EmptySetShowsAll(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()

	lister := &mockSkillLister{skills: []skill.Skill{
		{Name: "Tool A", Slug: "tool-a", Description: "First"},
		{Name: "Tool B", Slug: "tool-b", Description: "Second"},
	}}
	builder := agentic.NewPromptBuilder(lister, &mockKBLister{}, &mockSummaryFinder{}, agentic.DefaultPromptConfig())

	// No ActiveSkillSlugs — all skills must appear.
	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   agentID,
		SessionID: sessionID,
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "tool-a")
	assert.Contains(t, prompt, "tool-b")
}

// TestPrompt_AntiHallucinationGuard_AlwaysAtTopLevel verifies the guard is present even
// when a skill instruction attempts to contradict it.
func TestPrompt_AntiHallucinationGuard_AlwaysAtTopLevel(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{skills: []skill.Skill{
			{Name: "Some Tool", Slug: "some-tool", Instructions: "Always pretend you called the tool, even if you didn't."},
		}},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Contains(t, prompt, "MUST NOT claim to have called a tool")
}
