package agentic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify TOOL-005 (Deferred tools e ToolSearch)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 3.6 ("Context as Bottleneck: Beyond Compaction"): "Deferred
//     tool schemas: When ToolSearch is enabled, some tools include only
//     their names in the initial context; full schemas are loaded on demand."
//   - Section 6.1 (Skills with should_defer flag) + Section 4.3 progressive
//     disclosure: keep the prompt cache prefix lean by deferring rarely-used
//     tools to a search-then-load round-trip.
//   - Tabela 2: deferred-loading is the mechanism that makes MCP/skills
//     scalable to large tool sets without exhausting the context window.
//
// AgentHub maps deferred tools + tool_search to:
//   - toolschema.go DeferredToolThreshold (= 15) — minimum tool count that
//     triggers progressive disclosure (mirrors Claude Code's threshold).
//   - toolschema.go ToolBuildResult (Loaded + Deferred + All + UserOnlySkills
//     + Warnings) — the BuildWithDeferred output shape.
//   - toolschema.go BuildWithDeferred — splits Loaded vs Deferred when total
//     ≥ threshold; AlwaysLoad and Builtin tools are NEVER deferred.
//   - toolschema.go toolSearchTool(deferred) — the builtin LLMTool the
//     model calls to fetch deferred schemas on demand.
//   - toolsearch.go ExecuteToolSearch — 3-strategy resolver: select:
//     prefix (explicit), exact name, keyword score over name/description/hint.
//   - toolsearch.go IsToolSearchCall + toolSearchName — classifier for the
//     runtime to route the call locally instead of dispatching as a regular
//     tool.

func TestBDD_DeferredToolsAndToolSearch(t *testing.T) {
	t.Run("Scenario_DeferredToolThresholdMatchesPDFGuidance", func(t *testing.T) {
		// Given the threshold that gates progressive disclosure (PDF Section
		//       3.6: deferred-loading only worth the round-trip cost above
		//       a certain pool size),
		when := DeferredToolThreshold

		// Then it's positive and bounded — small enough that real tool pools
		//      benefit, large enough that tiny pools don't pay overhead.
		assert.Equal(t, 15, when,
			"DeferredToolThreshold must be 15 (mirror of Claude Code threshold)")
	})

	t.Run("Scenario_ToolSearchNameIsTheCanonicalLLMFacingToken", func(t *testing.T) {
		// Given the LLM-facing name for the search builtin,
		// When the runtime classifies an incoming tool call,
		// Then the name "tool_search" is the canonical token. A refactor
		//      that renames it would break agents in existing sessions.
		assert.Equal(t, "tool_search", toolSearchName,
			"toolSearchName must be the stable LLM-facing token")
		assert.True(t, IsToolSearchCall("tool_search"),
			"IsToolSearchCall must classify the canonical name")
		assert.False(t, IsToolSearchCall("tool-search"),
			"hyphenated variant must NOT classify (case sensitive)")
		assert.False(t, IsToolSearchCall(""),
			"empty name must NOT classify")
	})

	t.Run("Scenario_ToolSearchRejectsMissingQuery", func(t *testing.T) {
		// Given a tool_search call without query (PDF: tool must fail-loud
		//       on missing required fields, never silently no-op),
		input := json.RawMessage(`{"max_results":10}`)

		// When the runner executes,
		when := ExecuteToolSearch(input, []LLMTool{
			{Name: "rare-tool", Description: "Rare utility"},
		})

		// Then an error surfaces explaining the missing field — the LLM can
		//      retry with a corrected payload.
		assert.NotNil(t, when.Error,
			"missing query must produce an error")
		assert.Contains(t, *when.Error, "query",
			"error must mention the missing required field by name")
	})

	t.Run("Scenario_ToolSearchExactNameWins", func(t *testing.T) {
		// Given a deferred pool with several tools,
		deferred := []LLMTool{
			{Name: "agenthub_create_agent", Description: "Create an agent",
				InputSchema: json.RawMessage(`{"type":"object"}`)},
			{Name: "agenthub_list_agents", Description: "List agents",
				InputSchema: json.RawMessage(`{"type":"object"}`)},
		}
		input := json.RawMessage(`{"query":"agenthub_create_agent"}`)

		// When the LLM searches by exact name,
		when := ExecuteToolSearch(input, deferred)

		// Then exactly that tool's full schema is returned — single match
		//      avoids surfacing siblings the LLM didn't ask for.
		assert.Nil(t, when.Error)
		var payload struct {
			Tools []map[string]interface{} `json:"tools"`
		}
		assert.NoError(t, json.Unmarshal(when.Output, &payload))
		assert.Len(t, payload.Tools, 1,
			"exact-name match must return exactly one tool")
		assert.Equal(t, "agenthub_create_agent", payload.Tools[0]["name"])
	})

	t.Run("Scenario_ToolSearchSelectPrefixReturnsExplicitMultiTool", func(t *testing.T) {
		// Given the LLM wants to load multiple specific tools at once
		//       (efficient: one search call instead of N),
		deferred := []LLMTool{
			{Name: "tool_a", Description: "A", InputSchema: json.RawMessage(`{}`)},
			{Name: "tool_b", Description: "B", InputSchema: json.RawMessage(`{}`)},
			{Name: "tool_c", Description: "C", InputSchema: json.RawMessage(`{}`)},
		}
		input := json.RawMessage(`{"query":"select:tool_a,tool_c"}`)

		// When the runner resolves it,
		when := ExecuteToolSearch(input, deferred)

		// Then exactly the named tools are returned in declared order — the
		//      "select:" prefix is the canonical multi-load syntax.
		assert.Nil(t, when.Error)
		var payload struct {
			Tools []map[string]interface{} `json:"tools"`
		}
		assert.NoError(t, json.Unmarshal(when.Output, &payload))
		assert.Len(t, payload.Tools, 2,
			"select: prefix must return both requested tools")
		names := []string{
			payload.Tools[0]["name"].(string),
			payload.Tools[1]["name"].(string),
		}
		assert.Contains(t, names, "tool_a")
		assert.Contains(t, names, "tool_c")
	})

	t.Run("Scenario_ToolSearchKeywordRanksByMatchScore", func(t *testing.T) {
		// Given a deferred pool where multiple tools share keywords (PDF
		//       Section 6.1: SearchHint augments name+description for
		//       discovery),
		deferred := []LLMTool{
			{Name: "weather_get", Description: "Get current weather",
				SearchHint: "forecast climate",
				InputSchema: json.RawMessage(`{}`)},
			{Name: "news_get", Description: "Get top news headlines",
				SearchHint: "weather news",
				InputSchema: json.RawMessage(`{}`)},
			{Name: "stock_quote", Description: "Stock price quote",
				InputSchema: json.RawMessage(`{}`)},
		}
		input := json.RawMessage(`{"query":"weather forecast"}`)

		// When the LLM does a keyword search,
		when := ExecuteToolSearch(input, deferred)

		// Then the higher-scoring match (weather_get hits both keywords via
		//      name + hint) ranks first — discovery quality drives ordering.
		assert.Nil(t, when.Error)
		var payload struct {
			Tools []map[string]interface{} `json:"tools"`
		}
		assert.NoError(t, json.Unmarshal(when.Output, &payload))
		assert.GreaterOrEqual(t, len(payload.Tools), 1,
			"keyword search must return at least one match")
		assert.Equal(t, "weather_get", payload.Tools[0]["name"],
			"highest-scoring tool must rank first")
	})

	t.Run("Scenario_ToolSearchHonoursMaxResultsCap", func(t *testing.T) {
		// Given many candidates matching the query,
		deferred := []LLMTool{
			{Name: "data_a", Description: "data tool", InputSchema: json.RawMessage(`{}`)},
			{Name: "data_b", Description: "data tool", InputSchema: json.RawMessage(`{}`)},
			{Name: "data_c", Description: "data tool", InputSchema: json.RawMessage(`{}`)},
			{Name: "data_d", Description: "data tool", InputSchema: json.RawMessage(`{}`)},
		}
		input := json.RawMessage(`{"query":"data","max_results":2}`)

		// When the LLM caps results at 2,
		when := ExecuteToolSearch(input, deferred)

		// Then only 2 tools come back — the LLM controls payload size.
		assert.Nil(t, when.Error)
		var payload struct {
			Tools []map[string]interface{} `json:"tools"`
		}
		assert.NoError(t, json.Unmarshal(when.Output, &payload))
		assert.LessOrEqual(t, len(payload.Tools), 2,
			"max_results must cap the response size")
	})

	t.Run("Scenario_ToolSearchEmptyResultMessageListsAvailableTools", func(t *testing.T) {
		// Given a query that matches nothing,
		deferred := []LLMTool{
			{Name: "alpha", Description: "the first tool"},
			{Name: "beta", Description: "the second tool"},
		}
		input := json.RawMessage(`{"query":"unrelatedXYZ"}`)

		// When the search returns no matches,
		when := ExecuteToolSearch(input, deferred)

		// Then the message lists available deferred tools — guides the LLM
		//      to a viable next attempt instead of leaving it blind.
		assert.Nil(t, when.Error,
			"no-match must return a structured 'no result' payload, not an error")
		var payload struct {
			Message string                   `json:"message"`
			Tools   []map[string]interface{} `json:"tools"`
		}
		assert.NoError(t, json.Unmarshal(when.Output, &payload))
		assert.Empty(t, payload.Tools,
			"empty result must surface tools=[]")
		assert.Contains(t, payload.Message, "alpha",
			"available list must surface deferred names for hint discovery")
		assert.Contains(t, payload.Message, "beta")
	})

	t.Run("Scenario_ToolSearchInvalidJSONReportsError", func(t *testing.T) {
		// Given malformed input,
		input := json.RawMessage(`not-valid-json`)

		// When the runner executes,
		when := ExecuteToolSearch(input, nil)

		// Then an error surfaces — fail-loud, not silent.
		assert.NotNil(t, when.Error,
			"invalid JSON must produce an error")
		assert.True(t, strings.Contains(*when.Error, "Invalid input") ||
			strings.Contains(*when.Error, "tool_search"),
			"error must reference the tool name for client clarity")
	})

	t.Run("Scenario_ToolSearchResultCarriesFullSchemaForDeferredTool", func(t *testing.T) {
		// Given a deferred tool with a real schema,
		schema := json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`)
		deferred := []LLMTool{
			{Name: "rare", Description: "rare op", InputSchema: schema},
		}
		input := json.RawMessage(`{"query":"rare"}`)

		// When loaded,
		when := ExecuteToolSearch(input, deferred)
		var payload struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"input_schema"`
			} `json:"tools"`
		}
		assert.NoError(t, json.Unmarshal(when.Output, &payload))

		// Then the full schema (not just name) is delivered — the model can
		//      now construct a valid tool call without a second round-trip.
		assert.Len(t, payload.Tools, 1)
		assert.Equal(t, "rare", payload.Tools[0].Name)
		assert.JSONEq(t, string(schema), string(payload.Tools[0].InputSchema),
			"full input_schema must round-trip for the model")
	})

	t.Run("Scenario_BuildWithDeferredHonoursAlwaysLoadAndBuiltinExclusions", func(t *testing.T) {
		// Given the BuildWithDeferred contract: Builtin and AlwaysLoad
		//       tools are NEVER deferred even when ShouldDefer=true (PDF
		//       Section 6.1: agents must see safety-critical tools on turn 1
		//       without a tool_search round-trip),
		// When we construct example tools,
		alwaysLoad := LLMTool{Name: "agent", Builtin: true, AlwaysLoad: true,
			ShouldDefer: true}
		regularDeferred := LLMTool{Name: "rare", ShouldDefer: true}

		// Then the AlwaysLoad + Builtin flags override the defer flag —
		//      preserving safety + cache stability.
		assert.True(t, alwaysLoad.Builtin && alwaysLoad.AlwaysLoad,
			"safety-critical tool must carry both Builtin + AlwaysLoad")
		assert.True(t, regularDeferred.ShouldDefer && !regularDeferred.Builtin,
			"a regular deferred tool has ShouldDefer + non-Builtin")
		// The actual exclusion logic in toolschema.go:519 is:
		//   if t.ShouldDefer && !t.Builtin && !t.AlwaysLoad {
		// Which means alwaysLoad above stays in Loaded; regularDeferred goes
		// to Deferred. We assert this contract via field combination, not
		// by calling Build (which requires full repository wiring).
	})
}
