package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ContextEnvelopeAssembly(t *testing.T) {
	t.Run("Scenario_AssemblerProducesEnvelopeInCanonicalOrderRegardlessOfAppendOrder", func(t *testing.T) {
		// Given a builder appends sections in random order,
		// When Assemble runs,
		// Then the final envelope follows the canonical order from PDF
		// §7.2 (system → memory → rules → skills → tools → KB → recent
		// → aux → scratchpad), so consumers can rely on position.
		a := NewContextAssembler(DefaultAssemblerConfig())
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: "x"}))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionToolCatalog, Body: "tools"}))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionSystem, Body: "sys"}))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "mem"}))

		env, err := a.Assemble()
		require.NoError(t, err)
		assert.Equal(t, ContextSectionSystem, env.Sections[0].Kind)
		assert.Equal(t, ContextSectionMemory, env.Sections[1].Kind)
		assert.Equal(t, ContextSectionToolCatalog, env.Sections[2].Kind)
		assert.Equal(t, ContextSectionScratchpad, env.Sections[3].Kind)
	})

	t.Run("Scenario_FiftyToolCatalogDoesNotSqueezeOutMemoryOrKB", func(t *testing.T) {
		// Given a tenant has 50 tools (5000 tokens) and a memory section
		// (300 tokens),
		// When the assembler enforces the per-kind cap (e.g. 2000 tokens
		// for tool catalog),
		// Then the tool catalog is TRUNCATED rather than allowed to drown
		// out the memory section that follows it in the canonical order.
		cfg := DefaultAssemblerConfig()
		a := NewContextAssembler(cfg)
		require.NoError(t, a.Append(systemSection()))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "user prefers terse"}))
		fiftyTools := strings.Repeat("tool entry x ", 1000) // ~3000 tokens
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionToolCatalog, Body: fiftyTools}))

		env, err := a.Assemble()
		require.NoError(t, err)
		mem, hasMem := env.FindSection(ContextSectionMemory)
		require.True(t, hasMem, "memory survives even with huge tool catalog")
		assert.Contains(t, mem.Body, "user prefers terse")
		tools, _ := env.FindSection(ContextSectionToolCatalog)
		assert.True(t, tools.Truncated, "tool catalog must be truncated by cap")
	})

	t.Run("Scenario_BudgetExceededDropsLowestPriorityFirst", func(t *testing.T) {
		// Given total budget can't fit all sections,
		// When the assembler enforces budget,
		// Then it drops sections starting from the LOWEST priority
		// (scratchpad before aux before recent_messages) so the most
		// load-bearing context survives.
		cfg := AssemblerConfig{BudgetLimit: 30}
		a := NewContextAssembler(cfg)
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionSystem, Body: strings.Repeat("s", 40)}))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionRecentMessages, Body: strings.Repeat("r", 80)}))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("p", 80)}))

		env, _ := a.Assemble()
		scratch := findRawSection(env, ContextSectionScratchpad)
		require.NotNil(t, scratch)
		assert.True(t, scratch.Dropped, "scratchpad (lowest priority) dropped first")
	})

	t.Run("Scenario_SystemSectionIsNeverDroppedEvenWhenBudgetImpossible", func(t *testing.T) {
		// Given a budget so tight that even system alone exceeds it,
		// When assembler enforces budget,
		// Then system is preserved AND envelope is marked Truncated so
		// the caller knows the LLM call may be over-budget (caller
		// decides whether to fail or proceed with degraded context).
		cfg := AssemblerConfig{BudgetLimit: 5}
		a := NewContextAssembler(cfg)
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionSystem, Body: strings.Repeat("s", 200)}))

		env, _ := a.Assemble()
		sys, ok := env.FindSection(ContextSectionSystem)
		require.True(t, ok)
		assert.False(t, sys.Dropped)
		assert.True(t, env.Truncated, "caller alerted via Truncated flag")
	})

	t.Run("Scenario_DroppedSectionRecordsAuditableReason", func(t *testing.T) {
		// Given GOV-001 audits every context-shaping decision,
		// When a section is dropped,
		// Then DropReason carries the budget-exceeded message so audit
		// log shows WHY the agent received less context (not just
		// "section disappeared").
		cfg := AssemblerConfig{BudgetLimit: 20}
		a := NewContextAssembler(cfg)
		require.NoError(t, a.Append(systemSection()))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("x", 200)}))

		env, _ := a.Assemble()
		scratch := findRawSection(env, ContextSectionScratchpad)
		require.NotNil(t, scratch)
		assert.NotEmpty(t, scratch.DropReason)
	})

	t.Run("Scenario_AssembleIsDeterministicForSameInput", func(t *testing.T) {
		// Given the same set of section appends,
		// When Assemble runs twice,
		// Then output envelopes render identically (cache-friendly,
		// deterministic for tests + replay debugging).
		build := func() string {
			a := NewContextAssembler(DefaultAssemblerConfig())
			_ = a.Append(systemSection())
			_ = a.Append(ContextSection{Kind: ContextSectionMemory, Body: "facts"})
			_ = a.Append(ContextSection{Kind: ContextSectionToolCatalog, Body: "tools"})
			env, _ := a.Assemble()
			return env.Render()
		}
		assert.Equal(t, build(), build())
	})

	t.Run("Scenario_AssemblerRejectsDuplicateSectionKinds", func(t *testing.T) {
		// Given composition of memory facts happens BEFORE Append (the
		// memory hierarchy is collapsed first),
		// When the caller accidentally Appends two memory sections,
		// Then the second is rejected (forces caller to compose, not
		// have multiple sources of truth post-Append).
		a := NewContextAssembler(DefaultAssemblerConfig())
		require.NoError(t, a.Append(systemSection()))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "set A"}))
		err := a.Append(ContextSection{Kind: ContextSectionMemory, Body: "set B"})
		assert.Error(t, err)
	})

	t.Run("Scenario_AssemblerRejectsAssembleWithoutSystemSection", func(t *testing.T) {
		// Given every LLM call needs at least a system prompt,
		// When the caller forgets to Append system,
		// Then Assemble fails up-front (not silently producing a broken
		// envelope the LLM rejects later).
		a := NewContextAssembler(DefaultAssemblerConfig())
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionMemory, Body: "x"}))
		_, err := a.Assemble()
		assert.Error(t, err)
	})

	t.Run("Scenario_PerKindCapProtectsRecentMessagesFromBeingFlooded", func(t *testing.T) {
		// Given recent_messages has a large cap (16k by default),
		// When a long conversation history fills that cap,
		// Then recent_messages is preserved (not dropped) because its
		// canonical priority comes before aux/scratchpad which would
		// be dropped first.
		cfg := DefaultAssemblerConfig()
		a := NewContextAssembler(cfg)
		require.NoError(t, a.Append(systemSection()))
		bigConv := strings.Repeat("user: hi\nassistant: hi\n", 1000) // ~6k tokens
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionRecentMessages, Body: bigConv}))

		env, _ := a.Assemble()
		recent, ok := env.FindSection(ContextSectionRecentMessages)
		require.True(t, ok, "recent messages survive default budget")
		assert.False(t, recent.Dropped)
	})

	t.Run("Scenario_TotalTokensReflectsOnlySectionsActuallySentToLLM", func(t *testing.T) {
		// Given the runtime caches tokens-sent for cost tracking,
		// When sections are dropped by budget,
		// Then TotalTokens reflects ONLY the live sections (not the
		// pre-drop intent — billing must match what the LLM actually
		// received).
		cfg := AssemblerConfig{BudgetLimit: 20}
		a := NewContextAssembler(cfg)
		require.NoError(t, a.Append(systemSection()))
		require.NoError(t, a.Append(ContextSection{Kind: ContextSectionScratchpad, Body: strings.Repeat("x", 400)}))

		env, _ := a.Assemble()
		sys, _ := env.FindSection(ContextSectionSystem)
		assert.Equal(t, sys.EstimatedTokens, env.TotalTokens,
			"TotalTokens excludes dropped sections (cost-tracking accurate)")
	})
}
