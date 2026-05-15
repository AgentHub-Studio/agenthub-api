package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for BackgroundAgentSchedulingModelRegistry (FEAT-042).
// Grounded in arXiv:2604.14228v1 §10, §11.6, §12.3, §12.4.

// Scenario 1: Registry shape
// Given the default registry is created
// When I query all triggers
// Then I receive exactly 7 distinct profiles covering the full §10/§11.6/§12 taxonomy
func TestBDD_SchedulingRegistry_DefaultRegistry_HasSevenDistinctTriggers(t *testing.T) {
	// Given
	registry := NewBackgroundAgentSchedulingRegistry()

	// When
	all := registry.AllTriggers()

	// Then — count
	require.Len(t, all, 7, "expected 7 scheduling trigger profiles")

	// Then — all trigger IDs are distinct
	seen := make(map[BackgroundScheduleTrigger]bool)
	for _, p := range all {
		assert.False(t, seen[p.TriggerID], "duplicate trigger ID: %s", p.TriggerID)
		seen[p.TriggerID] = true
	}

	// Then — all seven expected IDs are present
	expectedIDs := []BackgroundScheduleTrigger{
		TriggerManual, TriggerCron, TriggerEvent, TriggerWebhook,
		TriggerKairosTick, TriggerThreshold, TriggerAbsenceTimeout,
	}
	for _, id := range expectedIDs {
		assert.True(t, seen[id], "missing trigger ID: %s", id)
	}
}

// Scenario 2: KAIROS proactive tick profile (§11.6)
// Given the registry is created
// When I look up the kairos_tick trigger
// Then it is polling, economically throttled, high resource budget,
//
//	has a 5-minute default interval, and does not require user presence
func TestBDD_SchedulingRegistry_KairosTick_HasCorrectProactiveSchedulingModel(t *testing.T) {
	// Given
	registry := NewBackgroundAgentSchedulingRegistry()

	// When
	profile, ok := registry.FindTriggerByID(TriggerKairosTick)

	// Then
	require.True(t, ok, "kairos_tick must be registered")
	assert.True(t, profile.IsPolling, "KAIROS tick must be a polling trigger")
	assert.True(t, profile.IsEconomicallyThrottled, "KAIROS must use SleepTool economic throttling")
	assert.Equal(t, ResourceBudgetHigh, profile.ResourceBudgetTier,
		"KAIROS inference calls carry high resource cost")
	assert.Equal(t, "5 minutes", profile.DefaultIntervalHint,
		"KAIROS default tick interval is 5 minutes per §11.6")
	assert.False(t, profile.RequiresUserPresence,
		"KAIROS runs autonomously when user is away")
	assert.True(t, profile.SupportsCheckpointing,
		"KAIROS long-horizon runs support file-history checkpoints")
	assert.Equal(t, "11.6", profile.PDFSection)
}

// Scenario 3: Manual trigger (§10 / Table 3)
// Given the registry is created
// When I query triggers requiring user presence
// Then only the manual trigger is returned,
//
//	and it is not a polling trigger and not economically throttled
func TestBDD_SchedulingRegistry_ManualTrigger_RequiresPresenceAndIsNotAutonomous(t *testing.T) {
	// Given
	registry := NewBackgroundAgentSchedulingRegistry()

	// When
	present := registry.TriggersRequiringUserPresence()

	// Then — only manual
	require.Len(t, present, 1)
	assert.Equal(t, TriggerManual, present[0].TriggerID)

	// Then — manual is not polling (user provides the explicit signal)
	assert.False(t, present[0].IsPolling)

	// Then — manual is not economically throttled (no autonomous wake-up loop)
	assert.False(t, present[0].IsEconomicallyThrottled)

	// Then — manual has low resource budget
	assert.Equal(t, ResourceBudgetLow, present[0].ResourceBudgetTier)
}

// Scenario 4: Polling vs event-driven partition
// Given the registry is created
// When I collect polling triggers and event-driven triggers
// Then they are disjoint and together cover all registered profiles
func TestBDD_SchedulingRegistry_PollingAndEventDriven_PartitionAllTriggers(t *testing.T) {
	// Given
	registry := NewBackgroundAgentSchedulingRegistry()

	// When
	polling := registry.PollingTriggers()
	eventDriven := registry.EventDrivenTriggers()

	// Then — no overlap
	pollingIDs := make(map[BackgroundScheduleTrigger]bool)
	for _, p := range polling {
		pollingIDs[p.TriggerID] = true
	}
	for _, p := range eventDriven {
		assert.False(t, pollingIDs[p.TriggerID],
			"trigger %s appears in both polling and event-driven", p.TriggerID)
	}

	// Then — union equals total
	assert.Equal(t, registry.Count(), len(polling)+len(eventDriven))

	// Then — cron/kairos_tick/threshold/absence_timeout are polling
	require.True(t, pollingIDs[TriggerCron])
	require.True(t, pollingIDs[TriggerKairosTick])
	require.True(t, pollingIDs[TriggerThreshold])
	require.True(t, pollingIDs[TriggerAbsenceTimeout])

	// Then — webhook/event/manual are event-driven (push or explicit)
	eventIDs := make(map[BackgroundScheduleTrigger]bool)
	for _, p := range eventDriven {
		eventIDs[p.TriggerID] = true
	}
	assert.True(t, eventIDs[TriggerWebhook])
	assert.True(t, eventIDs[TriggerEvent])
	assert.True(t, eventIDs[TriggerManual])
}

// Scenario 5: Registry invariants hold on the default seeding
// Given the default registry is created
// When each structural invariant is evaluated
// Then all invariants pass without error
func TestBDD_SchedulingRegistry_DefaultRegistry_AllInvariantsPass(t *testing.T) {
	// Given
	registry := NewBackgroundAgentSchedulingRegistry()

	// When / Then
	assert.NoError(t, registry.AtLeastOneTrigger(),
		"registry must contain at least one trigger")
	assert.NoError(t, registry.ManualTriggerRequiresUserPresence(),
		"manual trigger must require user presence")
	assert.NoError(t, registry.CheckpointingTriggersAreNotUserPresent(),
		"checkpointing triggers must not require user presence")
}

// Scenario 6: Checkpointing triggers are all autonomous long-horizon profiles
// Given the registry is created
// When I collect triggers that support checkpointing
// Then each one is not user-present and has a non-empty DefaultIntervalHint
func TestBDD_SchedulingRegistry_CheckpointingTriggers_AreAutonomousWithPollingInterval(t *testing.T) {
	// Given
	registry := NewBackgroundAgentSchedulingRegistry()

	// When
	checkpointed := registry.TriggersWithCheckpointing()

	// Then — at least one exists
	require.NotEmpty(t, checkpointed,
		"at least one trigger must support checkpointing for long-horizon runs")

	for _, p := range checkpointed {
		// Then — none require user presence
		assert.False(t, p.RequiresUserPresence,
			"trigger %s supports checkpointing so must be autonomous", p.TriggerID)

		// Then — all polling (checkpointing makes sense for repeating runs)
		assert.True(t, p.IsPolling,
			"trigger %s supports checkpointing and must be a polling trigger", p.TriggerID)

		// Then — all have a DefaultIntervalHint
		assert.NotEmpty(t, p.DefaultIntervalHint,
			"trigger %s supports checkpointing and must document its polling interval", p.TriggerID)
	}
}
