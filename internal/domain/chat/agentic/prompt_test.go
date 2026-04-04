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
)

// --- mocks ---

type mockSkillLister struct {
	skills []skill.Skill
}

func (m *mockSkillLister) ListByAgentID(_ context.Context, _ uuid.UUID) ([]skill.Skill, error) {
	return m.skills, nil
}

type mockKBLister struct {
	kbs []knowledgebase.KnowledgeBase
}

func (m *mockKBLister) ListByAgentID(_ context.Context, _ uuid.UUID) ([]knowledgebase.KnowledgeBase, error) {
	return m.kbs, nil
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
