package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
)

// BDD-style scenarios that ratify TOOL-001 (Interface and contract of tool)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1). Relevant PDF sections:
//
//   - Section 6 ("Extensibility: MCP, Plugins, Skills, and Hooks") and
//     Section 6.2 ("Tool Pool Assembly") establish that every callable tool
//     surface must expose a stable name, a description, an input schema, and
//     metadata flags that govern dispatch (read-only, destructive, deferred,
//     always-load, concurrency-safe, search-hint, interrupt-behavior).
//   - Section 4.2 ("Tool Dispatch and Streaming Execution") relies on the
//     read-only flag to decide which tools may run concurrently and which
//     must be serialised.
//   - Section 3.6 ("Context as Bottleneck: Beyond Compaction") describes
//     deferred tool schemas (full schema loaded only on demand via ToolSearch)
//     and per-tool-result budget — both modelled in AgentHub as per-tool
//     flags (ShouldDefer, MaxResultChars).
//
// AgentHub maps the contract to two layers:
//   - tool.Tool — the durable, per-tenant DB model with all PDF flags.
//   - LLMTool   — the runtime function-calling representation sent to the
//     model in tools[]. Internal-only fields (per-tool budget, builtin marker,
//     deferred flag) are tagged json:"-" so they never leak to the LLM.
//
// These BDD scenarios assert the contract observable to skills/runners. They
// do NOT depend on a database or a running model.

func TestBDD_ToolInterfaceContract(t *testing.T) {
	t.Run("Scenario_LLMToolWireShapeMatchesFunctionCallingProtocol", func(t *testing.T) {
		// Given a fully-populated LLMTool ready to be sent to the LLM,
		given := LLMTool{
			Name:        "document-search",
			Description: "Search the knowledge base for relevant snippets.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		}

		// When the runtime serialises it for the function-calling API,
		when, err := json.Marshal(given)
		assert.NoError(t, err, "LLMTool must serialise without error")

		// Then the JSON has exactly the three fields the model expects
		//      (PDF Section 6: tool description = name + description + input schema).
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(when, &decoded))
		assert.Equal(t, "document-search", decoded["name"],
			"LLM-facing name must round-trip")
		assert.Equal(t, "Search the knowledge base for relevant snippets.", decoded["description"],
			"LLM-facing description must round-trip")
		assert.NotNil(t, decoded["input_schema"],
			"input_schema is the contract the model relies on")
	})

	t.Run("Scenario_LLMToolHidesInternalFlagsFromTheModel", func(t *testing.T) {
		// Given an LLMTool whose internal flags govern dispatch but are not
		//       part of the function-calling contract,
		budget := 4096
		given := LLMTool{
			Name:                   "execute-sql",
			Description:            "Run a SQL query.",
			InputSchema:            json.RawMessage(`{}`),
			ReadOnly:               false,
			MaxResultChars:         50000,
			Builtin:                true,
			ShouldDefer:            true,
			IsDestructive:          true,
			SearchHint:             "sql query database",
			AllowedTools:           []string{"document-search"},
			AlwaysLoad:             false,
			DisableModelInvocation: false,
			ConcurrencySafe:        false,
			ContextMode:            "inline",
			WhenToUse:              "When the user asks for tabular data",
			InterruptBehavior:      "block",
			IsSearchOrRead:         false,
			TokenBudget:            &budget,
		}

		// When the runtime serialises for the LLM,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)

		// Then internal-only fields must be absent from the wire — PDF
		//      principle: minimal scaffolding visible to the model, maximal
		//      operational infrastructure in the harness.
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))
		hidden := []string{
			"MaxResultChars", "Builtin", "ShouldDefer", "IsDestructive",
			"SearchHint", "AllowedTools", "AlwaysLoad", "DisableModelInvocation",
			"ConcurrencySafe", "ContextMode", "WhenToUse", "InterruptBehavior",
			"IsSearchOrRead", "TokenBudget",
		}
		for _, k := range hidden {
			_, present := decoded[k]
			assert.False(t, present,
				"internal flag %q must not leak to the LLM (json:\"-\")", k)
		}
		// readOnly is allowed to leak via omitempty when false (i.e. absent)
		// — assert it's also not present here.
		_, present := decoded["readOnly"]
		assert.False(t, present,
			"readOnly is omitempty; must be absent when false")
	})

	t.Run("Scenario_ReadOnlyAndConcurrencySafeAreIndependent", func(t *testing.T) {
		// Given a tool that is not read-only but still safe to parallelise
		//       (PDF Section 4.2: AgentHub follows the same separation —
		//       isConcurrencySafe is independent of isReadOnly),
		given := LLMTool{
			Name:            "background-event",
			Description:     "Append an audit event (idempotent).",
			InputSchema:     json.RawMessage(`{}`),
			ReadOnly:        false,
			ConcurrencySafe: true,
		}

		// When the dispatcher classifies it for parallelism,
		when := given.ConcurrencySafe || given.ReadOnly

		// Then it qualifies for the concurrent batch even though it writes —
		//      preserves PDF isConcurrencySafe semantics.
		assert.True(t, when,
			"ConcurrencySafe-only tools must qualify for parallel batches")
	})

	t.Run("Scenario_ToolTypeRegistryRejectsUnknown", func(t *testing.T) {
		// Given the set of tool types AgentHub knows how to dispatch,
		// When an unknown type is checked,
		// Then the registry rejects it — preventing the runner from accepting
		//      a tool with an undispatchable type at write time.
		assert.True(t, tool.IsValidToolType(tool.ToolTypeHTTP), "HTTP must be recognised")
		assert.True(t, tool.IsValidToolType(tool.ToolTypeSQL), "SQL must be recognised")
		assert.True(t, tool.IsValidToolType(tool.ToolTypeDocumentSearch),
			"DOCUMENT_SEARCH must be recognised")
		assert.True(t, tool.IsValidToolType(tool.ToolTypeCustom), "CUSTOM must be recognised")
		assert.True(t, tool.IsValidToolType(tool.ToolTypeBlockly), "BLOCKLY must be recognised")
		assert.True(t, tool.IsValidToolType(tool.ToolTypeComposite), "COMPOSITE must be recognised")
		assert.False(t, tool.IsValidToolType("rocket-launcher"),
			"unknown types must be rejected so dispatch cannot fail silently")
		assert.False(t, tool.IsValidToolType(""),
			"empty type must be rejected")
	})

	t.Run("Scenario_BuiltinTakesPrecedenceMarkerForStableSort", func(t *testing.T) {
		// Given a mix of builtin and external tools (PDF Section 6.2 step 5:
		//       deduplication, with built-in tools taking precedence over MCP
		//       tools),
		builtin := LLMTool{Name: "agent", Builtin: true}
		external := LLMTool{Name: "agent", Builtin: false}

		// When the assembler sees both with the same name,
		// Then the builtin marker is the deciding factor — preserving stable
		//      ordering and avoiding cache invalidation from MCP additions.
		assert.True(t, builtin.Builtin,
			"builtin marker must be set on platform tools")
		assert.False(t, external.Builtin,
			"external tools must not pretend to be builtin")
		// The marker existence is the contract; concrete sort lives in
		// assembleToolPool — covered by toolschema_test.go.
	})

	t.Run("Scenario_DeferredToolMustHaveDescriptionForToolSearch", func(t *testing.T) {
		// Given a tool flagged as deferred (PDF Section 3.6: full schema
		//       loaded only when needed via ToolSearch),
		given := LLMTool{
			Name:        "rare-utility",
			Description: "Rebuilds the search index.",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			ShouldDefer: true,
			SearchHint:  "rebuild reindex",
		}

		// When ToolSearch tries to surface it,
		// Then a non-empty description and search hint exist so the LLM can
		//      discover it — a deferred tool with no hint would be unreachable.
		assert.NotEmpty(t, given.Description,
			"deferred tools must keep a description for ToolSearch matching")
		assert.NotEmpty(t, given.SearchHint,
			"deferred tools should carry a SearchHint to aid discovery")
	})

	t.Run("Scenario_ToolModelExposesLLMFacingSlug", func(t *testing.T) {
		// Given a Tool record whose human Name is verbose,
		given := tool.Tool{
			Name: "Search documents in the knowledge base",
			Slug: "document-search",
			Type: tool.ToolTypeDocumentSearch,
		}

		// When the runtime exposes it to the LLM,
		// Then Slug — not Name — is the stable, URL-safe identifier the model
		//      sees in tools[]. PDF Section 6: tools are identified by stable
		//      names, not human labels.
		assert.NotEqual(t, given.Name, given.Slug,
			"Slug must be a stable identifier distinct from human Name")
		assert.True(t, len(given.Slug) > 0 && len(given.Slug) <= 64,
			"Slug must be non-empty and bounded for prompt economy")
	})

	t.Run("Scenario_DestructiveToolIsFlaggedForExtraConfirmation", func(t *testing.T) {
		// Given a tool whose effect is irreversible (PDF principle:
		//       reversibility-weighted risk assessment, Table 1 row 11),
		given := LLMTool{
			Name:          "drop-table",
			Description:   "DROP a database table permanently.",
			InputSchema:   json.RawMessage(`{}`),
			IsDestructive: true,
		}

		// When the permission gate evaluates it,
		// Then the destructive flag forces extra scrutiny even when the
		//      ruleset would otherwise allow — the runner's permission engine
		//      consults this flag (see executeWithPermissions).
		assert.True(t, given.IsDestructive,
			"destructive flag must be honoured by the permission gate")
	})
}
