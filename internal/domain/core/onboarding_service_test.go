package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
)

// obChecklistStub fakes the checklist loader the OnboardingService depends on.
type obChecklistStub struct {
	items []core.CoreOnboardingChecklistItem
	err   error
}

func (s obChecklistStub) LoadCapabilityOnboardingChecklist(context.Context) ([]core.CoreOnboardingChecklistItem, error) {
	return s.items, s.err
}

// obSignalsStub fakes the per-tenant signal loader the OnboardingService depends on.
type obSignalsStub struct {
	signals core.OnboardingSignals
	err     error
}

func (s obSignalsStub) LoadSignals(context.Context) (core.OnboardingSignals, error) {
	return s.signals, s.err
}

// obFullChecklist mirrors the five seeded onboarding steps (steps 1-3 blocking).
func obFullChecklist() []core.CoreOnboardingChecklistItem {
	return []core.CoreOnboardingChecklistItem{
		{Slug: core.SeedOnboardingConnectLLMSlug, Title: "Connect LLM", StepOrder: 1, IsBlocking: true},
		{Slug: core.SeedOnboardingCreateAgentSlug, Title: "Create Agent", StepOrder: 2, IsBlocking: true},
		{Slug: core.SeedOnboardingAssignSkillSlug, Title: "Assign Skill", StepOrder: 3, IsBlocking: true},
		{Slug: core.SeedOnboardingConfigureKBSlug, Title: "Configure KB", StepOrder: 4, IsBlocking: false},
		{Slug: core.SeedOnboardingTestRunSlug, Title: "Test Run", StepOrder: 5, IsBlocking: false},
	}
}

func TestOnboardingService_Checklist_CarriesNoTenantState(t *testing.T) {
	svc := core.NewOnboardingService(obChecklistStub{items: obFullChecklist()}, obSignalsStub{})
	steps, err := svc.Checklist(context.Background())
	require.NoError(t, err)
	require.Len(t, steps, 5)
	for _, s := range steps {
		assert.False(t, s.Done, "Checklist must not carry per-tenant completion state")
	}
}

func TestOnboardingService_Status_FreshTenant_NothingDone(t *testing.T) {
	svc := core.NewOnboardingService(obChecklistStub{items: obFullChecklist()}, obSignalsStub{})
	status, err := svc.Status(context.Background())
	require.NoError(t, err)
	assert.False(t, status.Completed)
	require.Len(t, status.Steps, 5)
	for _, s := range status.Steps {
		assert.False(t, s.Done)
	}
}

func TestOnboardingService_Status_PerStepCompletion(t *testing.T) {
	svc := core.NewOnboardingService(
		obChecklistStub{items: obFullChecklist()},
		obSignalsStub{signals: core.OnboardingSignals{
			LLMConfigured:   true,
			HasUserAgent:    false,
			HasSkillBound:   true,
			HasKnowledgeBase: false,
			HasCompletedRun: true,
		}},
	)
	status, err := svc.Status(context.Background())
	require.NoError(t, err)

	done := map[string]bool{}
	for _, s := range status.Steps {
		done[s.Slug] = s.Done
	}
	assert.True(t, done[core.SeedOnboardingConnectLLMSlug])
	assert.False(t, done[core.SeedOnboardingCreateAgentSlug])
	assert.True(t, done[core.SeedOnboardingAssignSkillSlug])
	assert.False(t, done[core.SeedOnboardingConfigureKBSlug])
	assert.True(t, done[core.SeedOnboardingTestRunSlug])
	// Blocking step 2 (create-agent) is not done → overall not completed.
	assert.False(t, status.Completed)
}

func TestOnboardingService_Status_AllBlockingDone_IsCompleted(t *testing.T) {
	svc := core.NewOnboardingService(
		obChecklistStub{items: obFullChecklist()},
		obSignalsStub{signals: core.OnboardingSignals{
			LLMConfigured: true,
			HasUserAgent:  true,
			HasSkillBound: true,
			// configure-kb and test-run (non-blocking) intentionally left false.
		}},
	)
	status, err := svc.Status(context.Background())
	require.NoError(t, err)
	assert.True(t, status.Completed, "all blocking steps done → completed even if optional steps remain")
}

func TestOnboardingService_Status_DismissedFlag_IsCompleted(t *testing.T) {
	svc := core.NewOnboardingService(
		obChecklistStub{items: obFullChecklist()},
		obSignalsStub{signals: core.OnboardingSignals{Dismissed: true}},
	)
	status, err := svc.Status(context.Background())
	require.NoError(t, err)
	assert.True(t, status.Completed, "explicit dismiss → completed even with nothing else done")
}

func TestOnboardingService_Status_EmptyChecklist_NotVacuouslyCompleted(t *testing.T) {
	// ah_core catalog unavailable — the loader returns nil, nil (non-fatal).
	svc := core.NewOnboardingService(obChecklistStub{items: nil}, obSignalsStub{})
	status, err := svc.Status(context.Background())
	require.NoError(t, err)
	assert.False(t, status.Completed, "empty checklist must not report completed by vacuous truth")
	assert.Empty(t, status.Steps)
}

func TestOnboardingService_Status_ChecklistError(t *testing.T) {
	svc := core.NewOnboardingService(
		obChecklistStub{err: errors.New("ah_core unavailable")},
		obSignalsStub{},
	)
	_, err := svc.Status(context.Background())
	require.Error(t, err)
}

func TestOnboardingService_Status_SignalsError(t *testing.T) {
	svc := core.NewOnboardingService(
		obChecklistStub{items: obFullChecklist()},
		obSignalsStub{err: errors.New("db down")},
	)
	_, err := svc.Status(context.Background())
	require.Error(t, err)
}
