package agentic

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func systemSection() ContextSection {
	return ContextSection{Kind: ContextSectionSystem, Body: "You are an AgentHub assistant."}
}

func TestContextEnvelope_SectionKindEnumIsBounded(t *testing.T) {
	for _, k := range AllContextSectionKinds() {
		assert.True(t, IsValidContextSectionKind(k))
	}
	assert.False(t, IsValidContextSectionKind(ContextSectionKind("notebook")))
	assert.Equal(t, 9, len(AllContextSectionKinds()))
}

func TestContextEnvelope_CanonicalOrderIsStable(t *testing.T) {
	expected := []ContextSectionKind{
		ContextSectionSystem,
		ContextSectionMemory,
		ContextSectionRules,
		ContextSectionSkillCatalog,
		ContextSectionToolCatalog,
		ContextSectionKBSummary,
		ContextSectionRecentMessages,
		ContextSectionAuxPrompt,
		ContextSectionScratchpad,
	}
	assert.Equal(t, expected, AllContextSectionKinds())
}

func TestContextEnvelope_OnlySystemIsNonDroppable(t *testing.T) {
	assert.True(t, IsNonDroppable(ContextSectionSystem))
	for _, k := range AllContextSectionKinds() {
		if k == ContextSectionSystem {
			continue
		}
		assert.False(t, IsNonDroppable(k), "%q must be droppable", k)
	}
}

func TestContextEnvelope_Append_RejectsInvalidKind(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	err := a.Append(ContextSection{Kind: ContextSectionKind("nope"), Body: "x"})
	assert.True(t, errors.Is(err, ErrInvalidContextSectionKind))
}

func TestContextEnvelope_Append_RejectsDuplicateKind(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	require.NoError(t, a.Append(systemSection()))
	err := a.Append(systemSection())
	assert.True(t, errors.Is(err, ErrDuplicateSectionKind))
}

func TestContextEnvelope_Append_AutoEstimatesTokens(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	body := strings.Repeat("x", 400) // 400 chars / 4 = 100 tokens
	require.NoError(t, a.Append(systemSection()))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: body}))
	env, err := a.Assemble()
	require.NoError(t, err)
	mem, ok := env.FindSection(ContextSectionMemory)
	require.True(t, ok)
	assert.Equal(t, 100, mem.EstimatedTokens)
}

func TestContextEnvelope_Assemble_RequiresSystemSection(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "x"}))
	_, err := a.Assemble()
	assert.True(t, errors.Is(err, ErrSystemSectionRequired))
}

func TestContextEnvelope_Assemble_OrdersSectionsCanonically(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	// Append in REVERSE canonical order.
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: "scratch"}))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionAuxPrompt, Body: "aux"}))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "mem"}))
	require.NoError(t, a.Append(systemSection()))

	env, err := a.Assemble()
	require.NoError(t, err)
	require.Len(t, env.Sections, 4)
	assert.Equal(t, ContextSectionSystem, env.Sections[0].Kind)
	assert.Equal(t, ContextSectionMemory, env.Sections[1].Kind)
	assert.Equal(t, ContextSectionAuxPrompt, env.Sections[2].Kind)
	assert.Equal(t, ContextSectionScratchpad, env.Sections[3].Kind)
}

func TestContextEnvelope_Assemble_PerKindCapTruncatesBody(t *testing.T) {
	cfg := AssemblerConfig{
		BudgetLimit: 100000,
		PerKindCap:  map[ContextSectionKind]int{ContextSectionToolCatalog: 50},
	}
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(systemSection()))
	bigBody := strings.Repeat("t", 1000) // 250 tokens vs cap=50
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionToolCatalog, Body: bigBody}))

	env, err := a.Assemble()
	require.NoError(t, err)
	tools, ok := env.FindSection(ContextSectionToolCatalog)
	require.True(t, ok)
	assert.True(t, tools.Truncated)
	assert.Equal(t, 250, tools.OriginalTokens)
	assert.LessOrEqual(t, tools.EstimatedTokens, 51) // cap=50, allow rounding
}

func TestContextEnvelope_Assemble_BudgetEnforcementDropsLowestPriority(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 30}
	a := NewContextAssembler(cfg)
	// system: 8 tokens (32 chars), scratchpad: 50 tokens, recent_messages: 50 tokens.
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionSystem, Body: strings.Repeat("s", 32)}))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("p", 200)}))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionRecentMessages, Body: strings.Repeat("r", 200)}))

	env, err := a.Assemble()
	require.NoError(t, err)

	// System must always survive.
	_, hasSystem := env.FindSection(ContextSectionSystem)
	assert.True(t, hasSystem)

	// At least one non-system section must be dropped.
	assert.GreaterOrEqual(t, env.DroppedCount, 1)
	assert.True(t, env.Truncated)

	// First-dropped is the LOWEST canonical priority — scratchpad
	// before recent_messages.
	scratch := findRawSection(env, ContextSectionScratchpad)
	require.NotNil(t, scratch)
	assert.True(t, scratch.Dropped)
}

func findRawSection(e ContextEnvelope, k ContextSectionKind) *ContextSection {
	for i, s := range e.Sections {
		if s.Kind == k {
			return &e.Sections[i]
		}
	}
	return nil
}

func TestContextEnvelope_Assemble_NonDroppableNeverDropped(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 5} // tiny budget
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionSystem, Body: strings.Repeat("s", 200)}))

	env, err := a.Assemble()
	require.NoError(t, err)
	sys, ok := env.FindSection(ContextSectionSystem)
	assert.True(t, ok)
	assert.False(t, sys.Dropped, "system never dropped even when budget exceeded")
	assert.True(t, env.Truncated, "envelope marks truncated when budget impossible to meet")
}

func TestContextEnvelope_Assemble_ZeroBudgetDisablesEnforcement(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 0} // unlimited
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(systemSection()))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("x", 10000)}))

	env, err := a.Assemble()
	require.NoError(t, err)
	assert.Equal(t, 0, env.DroppedCount)
	assert.False(t, env.Truncated)
}

func TestContextEnvelope_Assemble_DropReasonIsRecorded(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 20}
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(systemSection()))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("x", 200)}))

	env, _ := a.Assemble()
	scratch := findRawSection(env, ContextSectionScratchpad)
	require.NotNil(t, scratch)
	require.True(t, scratch.Dropped)
	assert.Contains(t, scratch.DropReason, "budget exceeded")
}

func TestContextEnvelope_LiveSections_ExcludesDropped(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 20}
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(systemSection()))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("x", 200)}))

	env, _ := a.Assemble()
	live := env.LiveSections()
	for _, s := range live {
		assert.False(t, s.Dropped)
	}
}

func TestContextEnvelope_FindSection_IgnoresDropped(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 20}
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(systemSection()))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("x", 200)}))

	env, _ := a.Assemble()
	_, ok := env.FindSection(ContextSectionScratchpad)
	assert.False(t, ok, "FindSection must ignore dropped sections")
}

func TestContextEnvelope_Render_DeterministicCanonicalOrder(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionRecentMessages, Body: "hi"}))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "user prefers concise"}))
	require.NoError(t, a.Append(systemSection()))

	env, _ := a.Assemble()
	rendered := env.Render()
	// system comes before memory, memory before recent_messages.
	sysIdx := strings.Index(rendered, "AgentHub assistant")
	memIdx := strings.Index(rendered, "user prefers concise")
	recentIdx := strings.Index(rendered, "hi")
	require.Greater(t, sysIdx, -1)
	require.Greater(t, memIdx, -1)
	require.Greater(t, recentIdx, -1)
	assert.Less(t, sysIdx, memIdx)
	assert.Less(t, memIdx, recentIdx)
}

func TestContextEnvelope_Render_ReproducibleAcrossCalls(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	require.NoError(t, a.Append(systemSection()))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "x"}))

	env1, _ := a.Assemble()
	env2, _ := a.Assemble()
	assert.Equal(t, env1.Render(), env2.Render())
}

func TestContextEnvelope_Render_UsesTitleWhenPresent(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionSystem, Title: "## Custom System Header", Body: "x"}))
	env, _ := a.Assemble()
	assert.Contains(t, env.Render(), "# ## Custom System Header")
}

func TestContextEnvelope_TotalTokens_ReflectsLiveSectionsOnly(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 20}
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(systemSection()))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("x", 200)}))

	env, _ := a.Assemble()
	// TotalTokens excludes the dropped scratchpad — only system tokens count.
	sys, _ := env.FindSection(ContextSectionSystem)
	assert.Equal(t, sys.EstimatedTokens, env.TotalTokens)
}

func TestContextEnvelope_BudgetUsed_EqualsTotalTokens(t *testing.T) {
	a := NewContextAssembler(DefaultAssemblerConfig())
	require.NoError(t, a.Append(systemSection()))
	env, _ := a.Assemble()
	assert.Equal(t, env.TotalTokens, env.BudgetUsed)
}

func TestContextEnvelope_DefaultAssemblerConfig_HasSensibleDefaults(t *testing.T) {
	cfg := DefaultAssemblerConfig()
	assert.Greater(t, cfg.BudgetLimit, 0)
	assert.NotNil(t, cfg.PerKindCap)
	// system is not in PerKindCap (system is non-droppable but also not capped).
	_, hasSystem := cfg.PerKindCap[ContextSectionSystem]
	assert.False(t, hasSystem, "system should not have per-kind cap (it's the priority floor)")
	// catalogs/KB are capped.
	assert.Greater(t, cfg.PerKindCap[ContextSectionToolCatalog], 0)
	assert.Greater(t, cfg.PerKindCap[ContextSectionSkillCatalog], 0)
	assert.Greater(t, cfg.PerKindCap[ContextSectionKBSummary], 0)
}

func TestContextEnvelope_Render_OnlySystemWhenAllOthersDropped(t *testing.T) {
	cfg := AssemblerConfig{BudgetLimit: 30}
	a := NewContextAssembler(cfg)
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionSystem, Body: "SYSPROMPT_BODY"}))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("SCRATCH_MARKER ", 50)}))
	require.NoError(t, a.Append(ContextSection{Kind: ContextSectionAuxPrompt, Body: strings.Repeat("AUX_MARKER ", 50)}))

	env, _ := a.Assemble()
	rendered := env.Render()
	assert.Contains(t, rendered, "SYSPROMPT_BODY")
	assert.NotContains(t, rendered, "SCRATCH_MARKER")
	assert.NotContains(t, rendered, "AUX_MARKER")
}
