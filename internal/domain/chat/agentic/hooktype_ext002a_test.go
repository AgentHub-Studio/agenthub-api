package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// Tests for EXT-002a — 4 web-adapted hook events added to AllExtendedHookEvents.

func TestEXT002a_AllExtendedHookEvents_Is27(t *testing.T) {
	assert.Equal(t, 27, len(agentic.AllExtendedHookEvents),
		"AllExtendedHookEvents must be exactly 27 after EXT-002a")
}

func TestEXT002a_WebAdaptedEvents_Count(t *testing.T) {
	assert.Equal(t, 4, len(agentic.WebAdaptedHookEvents()))
}

func TestEXT002a_WebAdaptedEvents_ContainsExpected(t *testing.T) {
	set := map[agentic.ExtendedHookEvent]bool{}
	for _, e := range agentic.WebAdaptedHookEvents() {
		set[e] = true
	}
	assert.True(t, set[agentic.HookRunStartedExt])
	assert.True(t, set[agentic.HookRunCompleteExt])
	assert.True(t, set[agentic.HookContextWindowAlertExt])
	assert.True(t, set[agentic.HookKnowledgeBaseQueriedExt])
}

func TestEXT002a_IsWebAdaptedHookEvent_TrueForNew(t *testing.T) {
	assert.True(t, agentic.IsWebAdaptedHookEvent(agentic.HookRunStartedExt))
	assert.True(t, agentic.IsWebAdaptedHookEvent(agentic.HookRunCompleteExt))
	assert.True(t, agentic.IsWebAdaptedHookEvent(agentic.HookContextWindowAlertExt))
	assert.True(t, agentic.IsWebAdaptedHookEvent(agentic.HookKnowledgeBaseQueriedExt))
}

func TestEXT002a_IsWebAdaptedHookEvent_FalseForOriginal(t *testing.T) {
	// Original events must not be classified as web-adapted.
	assert.False(t, agentic.IsWebAdaptedHookEvent(agentic.HookPreToolUseExt))
	assert.False(t, agentic.IsWebAdaptedHookEvent(agentic.HookSessionStartExt))
	assert.False(t, agentic.IsWebAdaptedHookEvent(agentic.HookStopExt))
	assert.False(t, agentic.IsWebAdaptedHookEvent(agentic.HookSubagentStartExt))
	assert.False(t, agentic.IsWebAdaptedHookEvent(agentic.HookPermissionRequestExt))
}

func TestEXT002a_NewEventsAreValidHookEvents(t *testing.T) {
	// IsValidHookEvent must recognise all 4 new events.
	assert.True(t, agentic.IsValidHookEvent("RunStarted"))
	assert.True(t, agentic.IsValidHookEvent("RunComplete"))
	assert.True(t, agentic.IsValidHookEvent("ContextWindowAlert"))
	assert.True(t, agentic.IsValidHookEvent("KnowledgeBaseQueried"))
}

func TestEXT002a_NewEventsStringValues(t *testing.T) {
	assert.Equal(t, agentic.ExtendedHookEvent("RunStarted"), agentic.HookRunStartedExt)
	assert.Equal(t, agentic.ExtendedHookEvent("RunComplete"), agentic.HookRunCompleteExt)
	assert.Equal(t, agentic.ExtendedHookEvent("ContextWindowAlert"), agentic.HookContextWindowAlertExt)
	assert.Equal(t, agentic.ExtendedHookEvent("KnowledgeBaseQueried"), agentic.HookKnowledgeBaseQueriedExt)
}

func TestEXT002a_WebAdaptedSubsetOfAllEvents(t *testing.T) {
	allSet := map[agentic.ExtendedHookEvent]bool{}
	for _, e := range agentic.AllExtendedHookEvents {
		allSet[e] = true
	}
	for _, e := range agentic.WebAdaptedHookEvents() {
		assert.True(t, allSet[e], "web-adapted event %q must be in AllExtendedHookEvents", e)
	}
}

func TestEXT002a_AllExtendedHookEventsUnique(t *testing.T) {
	seen := map[agentic.ExtendedHookEvent]bool{}
	for _, e := range agentic.AllExtendedHookEvents {
		assert.False(t, seen[e], "duplicate event %q in AllExtendedHookEvents", e)
		seen[e] = true
	}
}

func TestEXT002a_AllHookCommandTypes_Count(t *testing.T) {
	assert.Equal(t, 4, len(agentic.AllHookCommandTypes()))
}

func TestEXT002a_AllHookCommandTypes_ContainsAll(t *testing.T) {
	set := map[agentic.HookCommandType]bool{}
	for _, t2 := range agentic.AllHookCommandTypes() {
		set[t2] = true
	}
	assert.True(t, set[agentic.HookCommandBash])
	assert.True(t, set[agentic.HookCommandPrompt])
	assert.True(t, set[agentic.HookCommandAgent])
	assert.True(t, set[agentic.HookCommandHTTP])
}

func TestEXT002a_IsValidHookCommandType_Valid(t *testing.T) {
	for _, typ := range agentic.AllHookCommandTypes() {
		assert.True(t, agentic.IsValidHookCommandType(typ),
			"expected %q to be valid", typ)
	}
}

func TestEXT002a_IsValidHookCommandType_Invalid(t *testing.T) {
	assert.False(t, agentic.IsValidHookCommandType(""))
	assert.False(t, agentic.IsValidHookCommandType("webhook"))
	assert.False(t, agentic.IsValidHookCommandType("Command"))
}

func TestEXT002a_OriginalCount_23EventsInOriginalSet(t *testing.T) {
	// All 4 web-adapted events must be absent from the original 23.
	// i.e. the set minus web-adapted must be exactly 23.
	webSet := map[agentic.ExtendedHookEvent]bool{}
	for _, e := range agentic.WebAdaptedHookEvents() {
		webSet[e] = true
	}
	original := 0
	for _, e := range agentic.AllExtendedHookEvents {
		if !webSet[e] {
			original++
		}
	}
	assert.Equal(t, 23, original,
		"AllExtendedHookEvents minus web-adapted must equal original 23")
}

// BDD scenarios for EXT-002a web-adapted hook events.

func TestBDD_EXT002a_WebAdaptedHookEvents(t *testing.T) {
	t.Run("Scenario_RunLifecycleEventsCoverFullRequestBoundary", func(t *testing.T) {
		// Given AgentHub runs on HTTP/SSE (not a CLI),
		// When an agent run is requested via POST /api/chat/sessions/{id}/run,
		// Then RunStarted fires at the beginning and RunComplete fires at the end,
		// giving hooks full coverage of the run lifecycle without needing CLI session hooks.
		set := map[agentic.ExtendedHookEvent]bool{}
		for _, e := range agentic.WebAdaptedHookEvents() {
			set[e] = true
		}
		assert.True(t, set[agentic.HookRunStartedExt], "RunStarted must cover run start boundary")
		assert.True(t, set[agentic.HookRunCompleteExt], "RunComplete must cover run end boundary")
	})

	t.Run("Scenario_ContextWindowAlertEnablesProactiveManagement", func(t *testing.T) {
		// Given an agent session can exhaust the context window silently,
		// When ContextWindowAlert fires at threshold (e.g. 80 % used),
		// Then a hook can summarize or compact BEFORE the window is exhausted,
		// giving operators proactive control over context budget.
		assert.True(t, agentic.IsWebAdaptedHookEvent(agentic.HookContextWindowAlertExt))
		assert.True(t, agentic.IsValidHookEvent(string(agentic.HookContextWindowAlertExt)))
	})

	t.Run("Scenario_KnowledgeBaseQueriedEnablesRAGAuditTrail", func(t *testing.T) {
		// Given LGPD/GDPR require audit trails for PII-containing knowledge bases,
		// When KnowledgeBaseQueried fires after each RAG vector-search,
		// Then operators can attach a webhook hook to log queries to ClickHouse
		// for compliance evidence without modifying the agent runtime.
		assert.True(t, agentic.IsWebAdaptedHookEvent(agentic.HookKnowledgeBaseQueriedExt))
		assert.True(t, agentic.IsValidHookEvent("KnowledgeBaseQueried"))
	})

	t.Run("Scenario_WebEventsDistinctFromCLIEvents", func(t *testing.T) {
		// Given Claude Code has CLI-specific session lifecycle (Start/End/Stop),
		// When adapting to AgentHub's HTTP runtime,
		// Then web-adapted events (RunStarted/RunComplete) are ADDITIONAL
		// to the original 23 — the original events are not reclassified.
		assert.False(t, agentic.IsWebAdaptedHookEvent(agentic.HookSessionStartExt),
			"SessionStart is original, not web-adapted")
		assert.False(t, agentic.IsWebAdaptedHookEvent(agentic.HookStopExt),
			"Stop is original, not web-adapted")

		// total = 23 original + 4 web-adapted
		assert.Equal(t, 27, len(agentic.AllExtendedHookEvents))
	})

	t.Run("Scenario_AllCommandTypesValidForWebHooks", func(t *testing.T) {
		// Given web operators need all 4 hook command variants,
		// When attaching handlers to web-specific events (RunComplete → http),
		// Then all command types (command, prompt, agent, http) are valid
		// — no partial set forces a workaround.
		types := agentic.AllHookCommandTypes()
		assert.Len(t, types, 4)
		for _, typ := range types {
			assert.True(t, agentic.IsValidHookCommandType(typ),
				"command type %q must be valid", typ)
		}
	})
}
