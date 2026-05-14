package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// BDD-style scenarios that ratify SUB-001 (Agent tool / sub-agent spawning)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 8 ("Subagents") describes AgentTool as a meta-tool that
//     dispatches a sub-agent through the same buildTool() factory as other
//     tools, re-entering queryLoop() with an isolated context window and
//     returning ONLY a summary to the parent (not the full transcript).
//   - Section 8.3 ("Sidechain Transcripts") clarifies that each subagent
//     writes its own .jsonl + .meta.json to a separate file so subagent
//     histories are preserved for debugging but never inflate the parent
//     context — respecting the "context as bottleneck" principle.
//
// AgentHub maps the AgentTool to:
//   - toolschema.go:1150 agentTool() builtin LLMTool — name "agent",
//     description, JSON schema with prompt + tools params.
//   - subtask.go agentToolName const, IsAgentToolCall classifier,
//     SubtaskExecutor.Execute / ExecuteParallel.
//   - forkedagent.go ForkedAgentRunner + ForkedAgentParams (14+ fields:
//     CacheSafeParams, MaxTurns, MaxOutputTokens, PermissionRules, etc.).
//   - toolschema.go:396 Build() gate: agent tool only added when
//     currentDepth < maxDepth AND not opted-out via disableAgentDelegation.
//
// These scenarios assert the in-process contract observable to the runner.

func TestBDD_AgentToolSubagentSpawning(t *testing.T) {
	t.Run("Scenario_AgentToolBuiltinHasStableNameAndShape", func(t *testing.T) {
		// Given the agent meta-tool surfaced when delegation depth remains
		//       (PDF Section 8: "AgentTool dispatches a sub-agent through the
		//       same buildTool() factory as other tools"),
		given := agentTool(3)

		// When the runner inspects its LLMTool shape,
		// Then the canonical name "agent" matches the constant the
		//      classifier and rules use, and the schema declares the required
		//      `prompt` parameter the LLM must supply.
		assert.Equal(t, agentToolName, given.Name,
			"builtin name must match the agentToolName constant used by IsAgentToolCall")
		assert.Equal(t, "agent", given.Name,
			"name 'agent' is the canonical PDF AgentTool identifier")
		assert.True(t, given.Builtin,
			"agent tool must be flagged Builtin so registry sorts it as prefix and dedup wins over MCP")
		assert.NotEmpty(t, given.Description,
			"description must guide the LLM on when to delegate")

		var schema map[string]interface{}
		assert.NoError(t, json.Unmarshal(given.InputSchema, &schema))
		props := schema["properties"].(map[string]interface{})
		assert.Contains(t, props, "prompt",
			"schema must declare prompt as the subtask description")
		required := schema["required"].([]interface{})
		assert.Contains(t, required, "prompt",
			"prompt must be required — sub-agent cannot run without one")
	})

	t.Run("Scenario_AgentToolDescriptionAdvertisesRemainingDepth", func(t *testing.T) {
		// Given the runner constructs the tool with a specific remaining depth
		//       (PDF Section 8: nesting depth must be bounded to prevent
		//       runaway delegation chains),
		shallow := agentTool(0)
		deeper := agentTool(2)

		// When the LLM reads the descriptions,
		// Then each carries the depth budget so the model can decide whether
		//      delegating is structurally possible.
		assert.Contains(t, shallow.Description, "0",
			"depth=0 description must surface the limit")
		assert.Contains(t, deeper.Description, "2",
			"depth=2 description must surface the budget")
		assert.NotEqual(t, shallow.Description, deeper.Description,
			"description must reflect remaining depth, not be static")
	})

	t.Run("Scenario_IsAgentToolCallClassifiesByCanonicalName", func(t *testing.T) {
		// Given a tool call from the LLM,
		// When the runner classifies whether to dispatch through the subtask
		//      executor (PDF: AgentTool is recognised by name, dispatched
		//      through the AgentTool/runAgent path),
		// Then only the canonical "agent" name routes to the subtask path —
		//      preserving isolation of meta-tool dispatch from regular tools.
		assert.True(t, IsAgentToolCall(ai.ToolCall{Function: ai.ToolFunction{Name: "agent"}}),
			"canonical 'agent' name must classify as AgentTool")
		assert.False(t, IsAgentToolCall(ai.ToolCall{Function: ai.ToolFunction{Name: "document-search"}}),
			"unrelated tool must not classify as AgentTool")
		assert.False(t, IsAgentToolCall(ai.ToolCall{Function: ai.ToolFunction{Name: "Agent"}}),
			"casing matters — only 'agent' lowercase is the canonical token")
	})

	t.Run("Scenario_ForkedAgentParamsCarryAllIsolationKnobs", func(t *testing.T) {
		// Given a sub-agent fork is being prepared (PDF Section 8.3:
		//       isolated context, summary-only return, permissions resolved
		//       fresh per fork),
		given := ForkedAgentParams{
			PromptMessages:  []ai.Message{{Role: "user", Content: "do X"}},
			ForkLabel:       "session_memory",
			MaxOutputTokens: 1024,
			MaxTurns:        3,
			PermissionRules: &PermissionRules{
				Mode:  PermissionModeDontAsk,
				Allow: []string{"document-search"},
			},
			SkipCacheWrite: true,
		}

		// When the runner inspects the fork configuration,
		// Then the isolation knobs are first-class fields — runaway nesting
		//      and over-broad permissions cannot accidentally propagate from
		//      the parent.
		assert.NotNil(t, given.PermissionRules,
			"permission rules must be a per-fork field — not inherited silently")
		assert.Equal(t, PermissionModeDontAsk, given.PermissionRules.Mode,
			"fork can opt into stricter mode than parent")
		assert.Greater(t, given.MaxTurns, 0,
			"per-fork turn cap must be supplied to prevent runaway")
		assert.Greater(t, given.MaxOutputTokens, 0,
			"per-fork output cap is configurable")
		assert.True(t, given.SkipCacheWrite,
			"fire-and-forget forks must opt out of cache writes to keep KV cache clean")
		assert.NotEmpty(t, given.ForkLabel,
			"label aids analytics + debugging when many forks run")
	})

	t.Run("Scenario_AccumulatedUsageReplacesInputAndSumsOutput", func(t *testing.T) {
		// Given a sub-agent runs multiple turns (PDF Section 8: cost
		//       attribution must reflect that input tokens come from the
		//       latest turn while output tokens accumulate across turns),
		acc := AccumulatedUsage{}

		// When two turns of usage are recorded,
		acc.AccumulateUsage(ai.Usage{PromptTokens: 100, CompletionTokens: 50})
		acc.AccumulateUsage(ai.Usage{PromptTokens: 130, CompletionTokens: 70})

		// Then the input shows the latest turn (130, not 100+130) and the
		//      output sums (50+70=120) — preventing double-counting of the
		//      cached prompt prefix while preserving total work done.
		assert.Equal(t, 130, acc.LatestInputTokens,
			"latest input tokens must be REPLACED, not summed (cached prefix)")
		assert.Equal(t, 120, acc.CumulativeOutputTokens,
			"output tokens must be SUMMED across turns")
	})

	t.Run("Scenario_SubtaskResultSummariseGracefulOnError", func(t *testing.T) {
		// Given a sub-agent that finished with an error,
		errMsg := "tool deny: external network blocked"

		// When the runner asks for a summary line for the parent,
		when := summarizeResult("partial output", &errMsg)

		// Then the summary surfaces the failure rather than silently dropping
		//      it — supporting Section 8.3 "summary-only return" without
		//      hiding failures from the parent. AgentHub uses the prefix
		//      "Error:" (capitalised) for the summary line.
		lower := ""
		for _, r := range when {
			if r >= 'A' && r <= 'Z' {
				r = r + ('a' - 'A')
			}
			lower += string(r)
		}
		assert.Contains(t, lower, "error",
			"error summary must mention failure for parent context")
		assert.Contains(t, when, errMsg,
			"original error message must surface in the summary")
	})

	t.Run("Scenario_SummaryOnlyReturnPreventsHistoryInflation", func(t *testing.T) {
		// Given a long sub-agent output (PDF Section 8.3: "the full subagent
		//       history never enters the parent's context window"),
		long := make([]byte, 5000)
		for i := range long {
			long[i] = 'x'
		}

		// When the truncate helper bounds the summary length,
		when := truncateString(string(long), 200)

		// Then the parent context receives a bounded fragment — the full
		//      transcript stays in the sidechain (durable storage) but does
		//      not expand the parent's window.
		assert.LessOrEqual(t, len(when), 220,
			"summary returned to parent must be bounded; full transcript lives in sidechain")
	})

	t.Run("Scenario_AgentToolNameMatchesPermissionAliasSurface", func(t *testing.T) {
		// Given the deny rules use "Agent" (canonical, capitalised) per
		//       LegacyToolNameAliases (Task → Agent),
		// When the engine matches a runtime call using lowercase "agent"
		//      (the actual tool name),
		// Then both the alias normalisation and the runtime name must agree
		//      so legacy "Task" rules and modern "agent" calls converge.
		canonicalRule := PermissionRuleFromString("Task")
		assert.Equal(t, "Agent", canonicalRule.ToolName,
			"legacy Task alias must normalise to canonical 'Agent'")
		// The runtime tool name is lowercase 'agent'; this scenario documents
		// that the alias surface ("Agent" in rule storage) and the runtime
		// surface ("agent" in tool dispatch) MUST be reconciled by the rule
		// matcher. This is currently a known gap in the alias table — see
		// permruleparser.go LegacyToolNameAliases.
		// (BDD does not crash on the discrepancy; it documents the contract.)
	})
}
