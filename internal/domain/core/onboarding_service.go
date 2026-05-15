package core

import "context"

// OnboardingStep is one checklist step plus the calling tenant's completion state.
type OnboardingStep struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ResourceType string `json:"resourceType"`
	StepOrder    int    `json:"stepOrder"`
	IsBlocking   bool   `json:"isBlocking"`
	IsAutomated  bool   `json:"isAutomated"`
	Done         bool   `json:"done"`
}

// OnboardingStatus is the aggregate onboarding state for a tenant: the full
// checklist with per-step completion, plus an overall completed flag.
type OnboardingStatus struct {
	Completed bool             `json:"completed"`
	Steps     []OnboardingStep `json:"steps"`
}

// onboardingChecklistLoader is the subset of CoreCapabilityOnboardingChecklistLoader
// the service needs — narrowed so it can be faked in unit tests.
type onboardingChecklistLoader interface {
	LoadCapabilityOnboardingChecklist(ctx context.Context) ([]CoreOnboardingChecklistItem, error)
}

// onboardingSignalLoader is the subset of OnboardingStatusRepository the service
// needs — narrowed so it can be faked in unit tests.
type onboardingSignalLoader interface {
	LoadSignals(ctx context.Context) (OnboardingSignals, error)
}

// OnboardingService aggregates the static onboarding checklist (from ah_core)
// with the calling tenant's per-step completion signals.
type OnboardingService struct {
	checklist onboardingChecklistLoader
	signals   onboardingSignalLoader
}

// NewOnboardingService wires an OnboardingService.
func NewOnboardingService(checklist onboardingChecklistLoader, signals onboardingSignalLoader) *OnboardingService {
	return &OnboardingService{checklist: checklist, signals: signals}
}

// Checklist returns the onboarding checklist definition with no per-tenant
// completion state (Done is always false).
func (s *OnboardingService) Checklist(ctx context.Context) ([]OnboardingStep, error) {
	items, err := s.checklist.LoadCapabilityOnboardingChecklist(ctx)
	if err != nil {
		return nil, err
	}
	steps := make([]OnboardingStep, len(items))
	for i, it := range items {
		steps[i] = stepFromItem(it, false)
	}
	return steps, nil
}

// Status returns the checklist with each step's per-tenant completion state and
// an overall completed flag. completed is true when every blocking step is done
// OR the tenant explicitly dismissed onboarding (settings.onboarding.completed).
// When the checklist catalog is unavailable (no blocking steps), completed
// reflects only the explicit dismiss flag — an empty checklist never reports
// "completed" by vacuous truth.
func (s *OnboardingService) Status(ctx context.Context) (OnboardingStatus, error) {
	items, err := s.checklist.LoadCapabilityOnboardingChecklist(ctx)
	if err != nil {
		return OnboardingStatus{}, err
	}
	sig, err := s.signals.LoadSignals(ctx)
	if err != nil {
		return OnboardingStatus{}, err
	}

	doneBySlug := map[string]bool{
		SeedOnboardingConnectLLMSlug:  sig.LLMConfigured,
		SeedOnboardingCreateAgentSlug: sig.HasUserAgent,
		SeedOnboardingAssignSkillSlug: sig.HasSkillBound,
		SeedOnboardingConfigureKBSlug: sig.HasKnowledgeBase,
		SeedOnboardingTestRunSlug:     sig.HasCompletedRun,
	}

	steps := make([]OnboardingStep, len(items))
	sawBlocking := false
	allBlockingDone := true
	for i, it := range items {
		done := doneBySlug[it.Slug]
		steps[i] = stepFromItem(it, done)
		if it.IsBlocking {
			sawBlocking = true
			if !done {
				allBlockingDone = false
			}
		}
	}

	return OnboardingStatus{
		Completed: sig.Dismissed || (sawBlocking && allBlockingDone),
		Steps:     steps,
	}, nil
}

func stepFromItem(it CoreOnboardingChecklistItem, done bool) OnboardingStep {
	return OnboardingStep{
		Slug:         it.Slug,
		Title:        it.Title,
		Description:  it.Description,
		ResourceType: it.ResourceType,
		StepOrder:    it.StepOrder,
		IsBlocking:   it.IsBlocking,
		IsAutomated:  it.IsAutomated,
		Done:         done,
	}
}
