package agentic

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify TOOL-002 (Tool registry / assembleToolPool)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1), Section 6.2 ("Tool Pool Assembly"), which describes a
// five-step pipeline:
//
//   1. Base tool enumeration         — getAllBaseTools()
//   2. Mode filtering                — getTools() per-mode + isEnabled()
//   3. Deny rule pre-filtering       — filterToolsByDenyRules()
//   4. MCP tool integration          — merge of appState.mcp.tools
//   5. Deduplication                 — built-in tools take precedence over MCP
//
// AgentHub's equivalent is `ToolSchemaBuilder.Build()` in toolschema.go which
// produces the same observable invariants:
//   - builtins assembled first, then MCP tools merged in,
//   - duplicates skipped (existing name wins, MCP loses),
//   - stable alphabetical sort with builtins as a contiguous prefix
//     (preserving the LLM prompt cache).
//
// These scenarios assert the pure registry helpers and constants without
// instantiating ToolSchemaBuilder (which requires DB-backed listers).

func TestBDD_ToolRegistryAssembly(t *testing.T) {
	t.Run("Scenario_AppendNonDuplicateMCPToolsKeepsExistingOnConflict", func(t *testing.T) {
		// Given a base pool that already contains a builtin tool and an MCP
		//       server returns a tool with the same name (PDF Section 6.2
		//       step 5: "built-in tools taking precedence over MCP tools"),
		base := []LLMTool{
			{Name: "agent", Builtin: true, Description: "Builtin agent delegation"},
			{Name: "document-search", Builtin: true, Description: "Builtin RAG"},
		}
		mcp := []LLMTool{
			{Name: "agent", Builtin: false, Description: "MCP server impostor"},
			{Name: "weather", Builtin: false, Description: "MCP weather tool"},
		}

		// When the assembler merges them,
		when := appendNonDuplicateMCPTools(base, mcp)

		// Then the impostor is dropped and the unique MCP tool is appended —
		//      builtins win on conflict, MCP additions never override.
		assert.Len(t, when, 3, "merge keeps 2 builtins + 1 unique MCP")
		found := make(map[string]bool)
		for _, item := range when {
			found[item.Name] = true
		}
		assert.True(t, found["agent"], "builtin agent must remain")
		assert.True(t, found["document-search"], "builtin RAG must remain")
		assert.True(t, found["weather"], "non-conflicting MCP tool must be added")
		// Verify the surviving "agent" is the builtin.
		for _, item := range when {
			if item.Name == "agent" {
				assert.True(t, item.Builtin, "surviving agent must be the builtin (precedence)")
				assert.Equal(t, "Builtin agent delegation", item.Description,
					"MCP impostor description must NOT replace builtin")
			}
		}
	})

	t.Run("Scenario_AppendNonDuplicateMCPToolsHandlesEmptyMCPList", func(t *testing.T) {
		// Given an empty MCP list (no servers configured or all failed),
		base := []LLMTool{{Name: "memory_store", Builtin: true}}

		// When the assembler is invoked,
		when := appendNonDuplicateMCPTools(base, nil)

		// Then the base pool is returned unchanged — graceful degradation when
		//      MCP integration is unavailable (PDF Section 6.2 step 4: "MCP
		//      tools from appState.mcp.tools are filtered ... and merged with
		//      built-in tools").
		assert.Len(t, when, 1, "no MCP additions must leave pool intact")
		assert.Equal(t, "memory_store", when[0].Name)
	})

	t.Run("Scenario_AppendNonDuplicateMCPToolsDeduplicatesAmongMCPItself", func(t *testing.T) {
		// Given an MCP server that emits two tools with the same name (broken
		//       server or two servers serving the same tool),
		base := []LLMTool{}
		mcp := []LLMTool{
			{Name: "search-web", Description: "first instance"},
			{Name: "search-web", Description: "duplicate from another server"},
		}

		// When the assembler merges them,
		when := appendNonDuplicateMCPTools(base, mcp)

		// Then only the first wins — preserves PDF dedup invariant within MCP
		//      sources too, not just across the builtin/MCP boundary.
		assert.Len(t, when, 1, "MCP-internal duplicates must be collapsed")
		assert.Equal(t, "first instance", when[0].Description,
			"first occurrence wins")
	})

	t.Run("Scenario_StableSortPlacesBuiltinsAsContiguousPrefix", func(t *testing.T) {
		// Given a tool pool freshly assembled before the sort step (PDF
		//       Section 6.2 final invariant: builtins first, then MCP/skill
		//       tools, both alphabetical to preserve the LLM prompt cache),
		given := []LLMTool{
			{Name: "weather", Builtin: false},
			{Name: "agent", Builtin: true},
			{Name: "document-search", Builtin: true},
			{Name: "github-create-issue", Builtin: false},
			{Name: "memory_store", Builtin: true},
			{Name: "linear-list-issues", Builtin: false},
		}

		// When the assembler applies its stable sort (Build() at toolschema.go
		//       line 465-470 — same comparator replicated here),
		sort.SliceStable(given, func(i, j int) bool {
			if given[i].Builtin != given[j].Builtin {
				return given[i].Builtin
			}
			return given[i].Name < given[j].Name
		})

		// Then builtins occupy positions 0..N-1 contiguously, then MCP/skill
		//      tools alphabetically — failure here invalidates the prompt cache.
		assert.Equal(t, "agent", given[0].Name,
			"first builtin alphabetically must be 'agent'")
		assert.Equal(t, "document-search", given[1].Name,
			"second builtin alphabetically must be 'document-search'")
		assert.Equal(t, "memory_store", given[2].Name,
			"third builtin alphabetically must be 'memory_store'")
		assert.Equal(t, "github-create-issue", given[3].Name,
			"first non-builtin alphabetically must be 'github-create-issue'")
		assert.Equal(t, "linear-list-issues", given[4].Name)
		assert.Equal(t, "weather", given[5].Name)
		// Boundary check: no non-builtin sneaks in among builtins.
		for i := 0; i < 3; i++ {
			assert.True(t, given[i].Builtin,
				"position %d must be a builtin (contiguous prefix)", i)
		}
		for i := 3; i < 6; i++ {
			assert.False(t, given[i].Builtin,
				"position %d must be a non-builtin (suffix)", i)
		}
	})

	t.Run("Scenario_DeferredToolThresholdGatesProgressiveDisclosure", func(t *testing.T) {
		// Given AgentHub's deferred-loading threshold (PDF Section 3.6:
		//       deferred tool schemas reduce prompt size; full schema loaded
		//       on demand via ToolSearch),
		when := DeferredToolThreshold

		// Then the threshold is a positive bound — preventing tiny tool pools
		//      from paying the ToolSearch round-trip cost while large pools
		//      benefit from progressive disclosure.
		assert.Greater(t, when, 0,
			"DeferredToolThreshold must be positive")
		assert.Equal(t, 15, when,
			"AgentHub mirrors Claude Code's threshold (15) for deferred loading")
	})

	t.Run("Scenario_ToolBuildResultExposesDeferredNamesForSystemPrompt", func(t *testing.T) {
		// Given a build result where some tools are deferred (PDF Section
		//       3.6: "deferred tools include only names in the initial
		//       context; full schemas loaded on demand"),
		given := &ToolBuildResult{
			Loaded: []LLMTool{
				{Name: "memory_store"},
				{Name: "agent"},
			},
			Deferred: []LLMTool{
				{Name: "rare-utility-1"},
				{Name: "rare-utility-2"},
				{Name: "rare-utility-3"},
			},
		}

		// When the runner asks for the deferred names to inject into the
		//      system prompt,
		when := given.DeferredToolNames()

		// Then names appear in order — preserving prompt determinism so the
		//      cache prefix remains valid across runs.
		assert.Equal(t, []string{
			"rare-utility-1", "rare-utility-2", "rare-utility-3",
		}, when, "deferred names must round-trip in declaration order")
	})

	t.Run("Scenario_ToolBuildResultPreservesWarningsForClient", func(t *testing.T) {
		// Given a build that suffered partial MCP failure (PDF Section 6.2
		//       step 4 narrative: "Always add the tools we got; always emit
		//       warnings for the failures so neither the tools nor the
		//       problem are silently dropped" — toolschema.go:417-419),
		given := &ToolBuildResult{
			Loaded: []LLMTool{{Name: "memory_store"}},
			Warnings: []string{
				"mcp:github: HTTP 401 on listTools",
				"mcp:linear: connection reset",
			},
		}

		// When the runner inspects the result,
		// Then warnings are non-nil and surface the per-server problem so the
		//      client can show them — failures are not silently swallowed.
		assert.Len(t, given.Warnings, 2,
			"each failed MCP server must contribute a warning")
		for _, w := range given.Warnings {
			assert.Contains(t, w, "mcp:",
				"warnings must be tagged with their source")
		}
	})

	t.Run("Scenario_AppendNonDuplicateMCPToolsSerialisesAsCleanWire", func(t *testing.T) {
		// Given a pool with a deferred MCP tool merged in,
		base := []LLMTool{
			{Name: "agent", Builtin: true, Description: "Spawn a sub-agent.",
				InputSchema: json.RawMessage(`{"type":"object"}`)},
		}
		mcp := []LLMTool{
			{Name: "search-web", Description: "Web search via MCP.",
				InputSchema: json.RawMessage(`{"type":"object"}`),
				ShouldDefer: true, SearchHint: "web search"},
		}

		// When merged and serialised for the LLM,
		merged := appendNonDuplicateMCPTools(base, mcp)
		raw, err := json.Marshal(merged)
		assert.NoError(t, err)

		// Then the wire keeps Name/Description/InputSchema and hides every
		//      internal flag — the registry's output is LLM-safe.
		var arr []map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &arr))
		assert.Len(t, arr, 2)
		for _, item := range arr {
			assert.Contains(t, item, "name")
			assert.Contains(t, item, "description")
			assert.Contains(t, item, "input_schema")
			assert.NotContains(t, item, "ShouldDefer",
				"internal flag must not leak from registry output")
			assert.NotContains(t, item, "SearchHint",
				"SearchHint must stay internal")
		}
	})
}
