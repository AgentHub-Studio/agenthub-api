package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- constructor and basic shape ---

func TestNewBackgroundAgentSchedulingRegistry_ReturnsNonNil(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	require.NotNil(t, r)
}

func TestBackgroundAgentSchedulingRegistry_Count_IsSevenProfiles(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.Equal(t, 7, r.Count())
}

func TestBackgroundAgentSchedulingRegistry_AllTriggers_LenMatchesCount(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.Len(t, r.AllTriggers(), r.Count())
}

func TestBackgroundAgentSchedulingRegistry_AllTriggers_ReturnsCopy(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	a := r.AllTriggers()
	b := r.AllTriggers()
	// Modifying one slice must not affect the other (independent copies).
	if len(a) > 0 {
		a[0].Label = "mutated"
		assert.NotEqual(t, "mutated", b[0].Label)
	}
}

// --- FindTriggerByID ---

func TestBackgroundAgentSchedulingRegistry_FindTriggerByID_KnownID_ReturnsProfile(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	cases := []BackgroundScheduleTrigger{
		TriggerManual,
		TriggerCron,
		TriggerEvent,
		TriggerWebhook,
		TriggerKairosTick,
		TriggerThreshold,
		TriggerAbsenceTimeout,
	}
	for _, id := range cases {
		t.Run(string(id), func(t *testing.T) {
			p, ok := r.FindTriggerByID(id)
			require.True(t, ok)
			assert.Equal(t, id, p.TriggerID)
		})
	}
}

func TestBackgroundAgentSchedulingRegistry_FindTriggerByID_UnknownID_ReturnsFalse(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	_, ok := r.FindTriggerByID("nonexistent_trigger")
	assert.False(t, ok)
}

// --- IsValidTriggerID ---

func TestBackgroundAgentSchedulingRegistry_IsValidTriggerID_KnownIDs_ReturnsTrue(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.True(t, r.IsValidTriggerID(TriggerCron))
	assert.True(t, r.IsValidTriggerID(TriggerKairosTick))
	assert.True(t, r.IsValidTriggerID(TriggerManual))
}

func TestBackgroundAgentSchedulingRegistry_IsValidTriggerID_UnknownID_ReturnsFalse(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.False(t, r.IsValidTriggerID("bogus"))
}

// --- PollingTriggers ---

func TestBackgroundAgentSchedulingRegistry_PollingTriggers_ContainsCronAndKairos(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	polling := r.PollingTriggers()
	ids := make(map[BackgroundScheduleTrigger]bool)
	for _, p := range polling {
		ids[p.TriggerID] = true
	}
	assert.True(t, ids[TriggerCron], "cron must be a polling trigger")
	assert.True(t, ids[TriggerKairosTick], "kairos_tick must be a polling trigger")
	assert.True(t, ids[TriggerThreshold], "threshold must be a polling trigger")
	assert.True(t, ids[TriggerAbsenceTimeout], "absence_timeout must be a polling trigger")
}

func TestBackgroundAgentSchedulingRegistry_PollingTriggers_DoesNotContainWebhookOrEvent(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	for _, p := range r.PollingTriggers() {
		assert.NotEqual(t, TriggerWebhook, p.TriggerID)
		assert.NotEqual(t, TriggerEvent, p.TriggerID)
	}
}

// --- EventDrivenTriggers ---

func TestBackgroundAgentSchedulingRegistry_EventDrivenTriggers_ContainsWebhookEventManual(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	driven := r.EventDrivenTriggers()
	ids := make(map[BackgroundScheduleTrigger]bool)
	for _, p := range driven {
		ids[p.TriggerID] = true
	}
	assert.True(t, ids[TriggerWebhook])
	assert.True(t, ids[TriggerEvent])
	assert.True(t, ids[TriggerManual])
}

func TestBackgroundAgentSchedulingRegistry_PollingAndEventDriven_AreComplementary(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	total := r.Count()
	polling := len(r.PollingTriggers())
	eventDriven := len(r.EventDrivenTriggers())
	assert.Equal(t, total, polling+eventDriven, "polling + event-driven must equal total")
}

// --- TriggersByType ---

func TestBackgroundAgentSchedulingRegistry_TriggersByType_Cron_HasThreeEntries(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	// cron type: TriggerCron, TriggerThreshold, TriggerAbsenceTimeout
	cronTyped := r.TriggersByType("cron")
	assert.Len(t, cronTyped, 3)
}

func TestBackgroundAgentSchedulingRegistry_TriggersByType_Proactive_ContainsKairos(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	proactive := r.TriggersByType("proactive")
	require.Len(t, proactive, 1)
	assert.Equal(t, TriggerKairosTick, proactive[0].TriggerID)
}

func TestBackgroundAgentSchedulingRegistry_TriggersByType_Unknown_ReturnsEmpty(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.Empty(t, r.TriggersByType("nonexistent_type"))
}

// --- TriggersRequiringUserPresence ---

func TestBackgroundAgentSchedulingRegistry_TriggersRequiringUserPresence_OnlyManual(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	present := r.TriggersRequiringUserPresence()
	require.Len(t, present, 1)
	assert.Equal(t, TriggerManual, present[0].TriggerID)
}

// --- TriggersWithCheckpointing ---

func TestBackgroundAgentSchedulingRegistry_TriggersWithCheckpointing_HasCronAndKairos(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	checkpointed := r.TriggersWithCheckpointing()
	ids := make(map[BackgroundScheduleTrigger]bool)
	for _, p := range checkpointed {
		ids[p.TriggerID] = true
	}
	assert.True(t, ids[TriggerCron])
	assert.True(t, ids[TriggerKairosTick])
}

func TestBackgroundAgentSchedulingRegistry_CheckpointingTriggers_DoNotRequireUserPresence(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	for _, p := range r.TriggersWithCheckpointing() {
		assert.False(t, p.RequiresUserPresence,
			"checkpointing trigger %s must not require user presence", p.TriggerID)
	}
}

// --- LeastResourceIntensiveTrigger ---

func TestBackgroundAgentSchedulingRegistry_LeastResourceIntensiveTrigger_ReturnsLowBudget(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	p, ok := r.LeastResourceIntensiveTrigger()
	require.True(t, ok)
	assert.Equal(t, ResourceBudgetLow, p.ResourceBudgetTier)
}

// --- profile content assertions ---

func TestBackgroundAgentSchedulingRegistry_KairosTick_IsEconomicallyThrottled(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	p, ok := r.FindTriggerByID(TriggerKairosTick)
	require.True(t, ok)
	assert.True(t, p.IsEconomicallyThrottled)
}

func TestBackgroundAgentSchedulingRegistry_KairosTick_DefaultIntervalHintFiveMinutes(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	p, ok := r.FindTriggerByID(TriggerKairosTick)
	require.True(t, ok)
	assert.Equal(t, "5 minutes", p.DefaultIntervalHint)
}

func TestBackgroundAgentSchedulingRegistry_KairosTick_ResourceBudgetHigh(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	p, ok := r.FindTriggerByID(TriggerKairosTick)
	require.True(t, ok)
	assert.Equal(t, ResourceBudgetHigh, p.ResourceBudgetTier)
}

func TestBackgroundAgentSchedulingRegistry_ManualTrigger_NotEconomicallyThrottled(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	p, ok := r.FindTriggerByID(TriggerManual)
	require.True(t, ok)
	assert.False(t, p.IsEconomicallyThrottled)
}

func TestBackgroundAgentSchedulingRegistry_AllProfiles_HaveNonEmptyLabel(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	for _, p := range r.AllTriggers() {
		assert.NotEmpty(t, p.Label, "trigger %s must have a non-empty label", p.TriggerID)
	}
}

func TestBackgroundAgentSchedulingRegistry_AllProfiles_HaveNonEmptyDescription(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	for _, p := range r.AllTriggers() {
		assert.NotEmpty(t, p.Description, "trigger %s must have a non-empty description", p.TriggerID)
	}
}

func TestBackgroundAgentSchedulingRegistry_AllProfiles_HaveNonEmptyPDFSection(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	for _, p := range r.AllTriggers() {
		assert.NotEmpty(t, p.PDFSection, "trigger %s must reference a PDF section", p.TriggerID)
	}
}

// --- invariants ---

func TestBackgroundAgentSchedulingRegistry_AtLeastOneTrigger_Passes(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.NoError(t, r.AtLeastOneTrigger())
}

func TestBackgroundAgentSchedulingRegistry_ManualTriggerRequiresUserPresence_Passes(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.NoError(t, r.ManualTriggerRequiresUserPresence())
}

func TestBackgroundAgentSchedulingRegistry_CheckpointingTriggersAreNotUserPresent_Passes(t *testing.T) {
	r := NewBackgroundAgentSchedulingRegistry()
	assert.NoError(t, r.CheckpointingTriggersAreNotUserPresent())
}

func TestBackgroundAgentSchedulingRegistry_AtLeastOneTrigger_FailsOnEmpty(t *testing.T) {
	r := &BackgroundAgentSchedulingRegistry{}
	assert.Error(t, r.AtLeastOneTrigger())
}

func TestBackgroundAgentSchedulingRegistry_ManualTriggerRequiresUserPresence_VacuouslyTrueWhenAbsent(t *testing.T) {
	// No manual trigger — invariant passes vacuously.
	r := &BackgroundAgentSchedulingRegistry{
		profiles: []BackgroundAgentScheduleProfile{
			{TriggerID: TriggerCron, Label: "c", Description: "d", PDFSection: "12.3",
				TriggerType: "cron", RequiresUserPresence: false, ResourceBudgetTier: ResourceBudgetLow},
		},
	}
	assert.NoError(t, r.ManualTriggerRequiresUserPresence())
}
