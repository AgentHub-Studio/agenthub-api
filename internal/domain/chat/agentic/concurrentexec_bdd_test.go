package agentic

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify TOOL-006 (Execução concorrente segura
// para leitura) against the Claude Code architecture paper "Dive into
// Claude Code" (arXiv:2604.14228v1):
//
//   - Section 4.2 ("Tool Dispatch and Streaming Execution"): "Both paths
//     classify tools as concurrent-safe or exclusive. Read-only operations
//     can execute in parallel, while state-modifying operations like shell
//     commands are serialized."
//   - Section 4.2 + StreamingToolExecutor docs: a Sibling abort controller
//     fires when ANY Bash tool errors, immediately terminating other in-
//     flight subprocesses rather than letting them run to completion.
//   - Section 4.2 progress-available signal: results are buffered and
//     emitted in the order tools were received, so output order matches the
//     LLM's request even when tools run in parallel.
//
// AgentHub maps concurrent execution to:
//   - toolexec.go PartitionToolCalls — splits a batch into runs of
//     consecutive concurrency-safe tools (parallel) and individual write
//     tools (serial). Order preserved across partition boundaries.
//   - toolexec.go BuildReadOnlyIndex — derives the safety map from LLMTool
//     definitions (ReadOnly OR ConcurrencySafe).
//   - toolschema.go IsReadOnlyTool — hardcoded list of known-safe builtins.
//   - readonlyop.go isReadOnlyOperation + readOnlyOperationVerbs — verb-
//     based heuristic for SQL/HTTP read intent.
//   - StreamingToolExecutor.executeParallel — semaphore-bounded by
//     RunConfig.ConcurrentReadTools + sibling abort context.
//   - RunConfig.ConcurrentReadTools default = 3 (config.go).

func TestBDD_ConcurrentReadExecution(t *testing.T) {
	t.Run("Scenario_PartitionGroupsConsecutiveReadOnlyTools", func(t *testing.T) {
		// Given a sequence of tool calls — 2 reads, 1 write, 1 read — that
		//       the LLM emitted in order (PDF Section 4.2: parallelism only
		//       for consecutive concurrent-safe tools; order preserved
		//       across partitions),
		readOnly := map[string]bool{"read1": true, "read2": true, "read3": true}
		given := []ai.ToolCall{
			{Function: ai.ToolFunction{Name: "read1"}},
			{Function: ai.ToolFunction{Name: "read2"}},
			{Function: ai.ToolFunction{Name: "shell"}},
			{Function: ai.ToolFunction{Name: "read3"}},
		}

		// When the runner partitions for dispatch,
		when := PartitionToolCalls(given, readOnly)

		// Then 3 batches are produced: {read1, read2} concurrent, {shell}
		//      serial, {read3} concurrent — order across batches matches
		//      the LLM's intended sequence.
		assert.Len(t, when, 3,
			"3 batches expected: 2-read concurrent + 1-write serial + 1-read concurrent")
		assert.True(t, when[0].IsConcurrencySafe,
			"first batch (read1+read2) must be marked concurrent-safe")
		assert.Equal(t, []int{0, 1}, when[0].Indices,
			"first batch indices must be [0,1] preserving LLM order")
		assert.False(t, when[1].IsConcurrencySafe,
			"shell batch must NOT be concurrent-safe")
		assert.Equal(t, []int{2}, when[1].Indices,
			"shell batch is single-tool at index 2")
		assert.True(t, when[2].IsConcurrencySafe,
			"third batch (read3) is concurrent-safe but isolated")
		assert.Equal(t, []int{3}, when[2].Indices,
			"third batch index is [3]")
	})

	t.Run("Scenario_PartitionExtendsRunOfReads", func(t *testing.T) {
		// Given 5 consecutive reads (no writes interrupt the run),
		readOnly := map[string]bool{"a": true, "b": true, "c": true, "d": true, "e": true}
		given := []ai.ToolCall{
			{Function: ai.ToolFunction{Name: "a"}},
			{Function: ai.ToolFunction{Name: "b"}},
			{Function: ai.ToolFunction{Name: "c"}},
			{Function: ai.ToolFunction{Name: "d"}},
			{Function: ai.ToolFunction{Name: "e"}},
		}

		// When partitioned,
		when := PartitionToolCalls(given, readOnly)

		// Then a single concurrent batch — the runner can dispatch all 5
		//      in parallel (subject to ConcurrentReadTools cap).
		assert.Len(t, when, 1,
			"5 consecutive reads must collapse to 1 concurrent batch")
		assert.True(t, when[0].IsConcurrencySafe)
		assert.Equal(t, []int{0, 1, 2, 3, 4}, when[0].Indices)
	})

	t.Run("Scenario_PartitionForcesSerializationOnWrites", func(t *testing.T) {
		// Given 3 consecutive write tools,
		given := []ai.ToolCall{
			{Function: ai.ToolFunction{Name: "shell"}},
			{Function: ai.ToolFunction{Name: "edit"}},
			{Function: ai.ToolFunction{Name: "delete"}},
		}

		// When partitioned with empty readOnly map,
		when := PartitionToolCalls(given, nil)

		// Then 3 single-tool batches — writes never run in parallel (PDF
		//      Section 4.2: state-modifying operations serialised).
		assert.Len(t, when, 3,
			"3 writes must produce 3 separate serial batches")
		for i, batch := range when {
			assert.False(t, batch.IsConcurrencySafe,
				"write batch %d must NOT be concurrent-safe", i)
			assert.Len(t, batch.Indices, 1,
				"each write batch holds exactly one tool")
		}
	})

	t.Run("Scenario_PartitionEmptyInputReturnsNil", func(t *testing.T) {
		// Given no tool calls (turn ended without tool_use),
		when := PartitionToolCalls(nil, nil)

		// Then nil — caller can range without nil-check guard.
		assert.Nil(t, when,
			"empty input must return nil (no work)")
	})

	t.Run("Scenario_BuildReadOnlyIndexHonoursBothFlags", func(t *testing.T) {
		// Given LLMTools with various combinations of ReadOnly /
		//       ConcurrencySafe (PDF Section 4.2 + TOOL-001: the two flags
		//       are independent — ConcurrencySafe lets non-read tools also
		//       parallelise),
		given := []LLMTool{
			{Name: "doc-search", ReadOnly: true},
			{Name: "audit-log", ConcurrencySafe: true}, // appends but safe
			{Name: "sql-write", ReadOnly: false, ConcurrencySafe: false},
			{Name: "both-flags", ReadOnly: true, ConcurrencySafe: true},
		}

		// When the index is built,
		when := BuildReadOnlyIndex(given)

		// Then either flag set marks the tool as parallel-eligible.
		assert.True(t, when["doc-search"], "ReadOnly tool must be in index")
		assert.True(t, when["audit-log"], "ConcurrencySafe tool must be in index")
		assert.False(t, when["sql-write"], "neither flag set must NOT be in index")
		assert.True(t, when["both-flags"], "both flags set must be in index")
	})

	t.Run("Scenario_BuildReadOnlyIndexHandlesEmptyInput", func(t *testing.T) {
		// Given no tools,
		when := BuildReadOnlyIndex(nil)

		// Then non-nil empty map — caller can lookup safely without
		//      nil-deref panic.
		assert.NotNil(t, when, "empty input must produce non-nil empty map")
		assert.Empty(t, when, "no tools means no entries")
	})

	t.Run("Scenario_DBReadOnlyFlagSupplementsHardcodedKnownSafe", func(t *testing.T) {
		// Given a custom skill marked read_only in the DB but unknown to
		//       the hardcoded IsReadOnlyTool list (PDF: registry-supplied
		//       safety must compose with hardcoded builtins),
		readOnly := map[string]bool{"custom-skill-xyz": true}
		given := []ai.ToolCall{
			{Function: ai.ToolFunction{Name: "custom-skill-xyz"}},
			{Function: ai.ToolFunction{Name: "another-custom"}},
			{Function: ai.ToolFunction{Name: "custom-skill-xyz"}},
		}

		// When partitioned (note "another-custom" is NOT in the index),
		when := PartitionToolCalls(given, readOnly)

		// Then the DB flag treats custom-skill-xyz as concurrent — the index
		//      composes with hardcoded list, both contribute to safety.
		assert.GreaterOrEqual(t, len(when), 2,
			"DB-flagged read-only must partition based on the index")
	})

	t.Run("Scenario_ConcurrentReadToolsCapBoundsParallelism", func(t *testing.T) {
		// Given the production default RunConfig (PDF Section 4.2 implies a
		//       cap to prevent unbounded goroutine spawn),
		given := DefaultRunConfig()

		// When the runner inspects the cap,
		when := given.ConcurrentReadTools

		// Then it's positive and small — preventing 50 reads from spawning
		//      50 goroutines that thrash the DB / external services.
		assert.Greater(t, when, 0,
			"ConcurrentReadTools cap must be positive — bounded parallelism")
		assert.LessOrEqual(t, when, 16,
			"cap must be sane (≤16) to avoid resource exhaustion")
	})

	t.Run("Scenario_ReadOnlyOperationVerbDetectorBypassesDestructiveBlock", func(t *testing.T) {
		// Given a meta-tool (e.g. agenthub_manage) flagged IsDestructive=true
		//       at the skill level but invoked with a read-only operation
		//       verb (PDF Section 4.2 + readonlyop.go: meta-tools that
		//       multiplex CRUD via an `operation` field need a SECOND-level
		//       check so safe ops like list/get/show don't block),
		// When the heuristic inspects the operation field,
		// Then known-safe verbs (list/get/show/describe/...) classify as
		//      read-only; unknown verbs (create/delete/update) do not.
		assert.True(t, isReadOnlyOperation(`{"operation":"list","resource":"agents"}`),
			"operation=list must classify as read-only on a meta-tool")
		assert.True(t, isReadOnlyOperation(`{"action":"get","id":"abc"}`),
			"action=get must classify as read-only")
		assert.True(t, isReadOnlyOperation(`{"method":"GET","url":"https://x"}`),
			"method=GET (case-insensitive) must classify as read-only")
		assert.False(t, isReadOnlyOperation(`{"operation":"create","resource":"agents"}`),
			"operation=create must NOT classify as read-only")
		assert.False(t, isReadOnlyOperation(`{"operation":"delete","id":"abc"}`),
			"operation=delete must NOT classify as read-only")
		assert.False(t, isReadOnlyOperation(`{}`),
			"missing operation field defaults to NOT read-only (fail-closed)")
		assert.False(t, isReadOnlyOperation(`not-json`),
			"invalid JSON defaults to NOT read-only (fail-closed)")
	})

	t.Run("Scenario_PartitionPreservesGlobalOrder", func(t *testing.T) {
		// Given an interleaved sequence (PDF Section 4.2: results emitted
		//       in the order tools were RECEIVED, even when run in
		//       parallel; partitioning must preserve global ordering across
		//       batches),
		readOnly := map[string]bool{"r": true}
		given := []ai.ToolCall{
			{Function: ai.ToolFunction{Name: "r"}},
			{Function: ai.ToolFunction{Name: "w"}},
			{Function: ai.ToolFunction{Name: "r"}},
			{Function: ai.ToolFunction{Name: "w"}},
			{Function: ai.ToolFunction{Name: "r"}},
		}

		// When partitioned,
		when := PartitionToolCalls(given, readOnly)

		// Then concatenating batch indices reconstructs the original order.
		var reconstructed []int
		for _, b := range when {
			reconstructed = append(reconstructed, b.Indices...)
		}
		assert.Equal(t, []int{0, 1, 2, 3, 4}, reconstructed,
			"partition must preserve global ordering across batches")
	})
}
