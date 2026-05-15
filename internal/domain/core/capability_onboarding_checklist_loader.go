package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityOnboardingChecklistLoader loads tenant onboarding checklist
// steps from ah_core.capability_onboarding_checklist. These 5 rows (seeded
// by migration 000106) define a guided onboarding path for new tenants,
// walking them from zero to their first successful agent chat session:
//
//   - onboarding-connect-llm   (step 1, blocking): configure LLM API key
//   - onboarding-create-agent  (step 2, blocking): create a capability agent
//   - onboarding-assign-skill  (step 3, blocking): bind a capability skill
//   - onboarding-configure-kb  (step 4, optional): create a knowledge base
//   - onboarding-test-run      (step 5, optional): run first chat session
//
// Non-fatal when the ah_core schema or the capability_onboarding_checklist
// table is missing — supports fresh deployments where migration 000106 has
// not yet run.
type CoreCapabilityOnboardingChecklistLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityOnboardingChecklistLoader creates a
// CoreCapabilityOnboardingChecklistLoader backed by pool.
func NewCoreCapabilityOnboardingChecklistLoader(pool *pgxpool.Pool) *CoreCapabilityOnboardingChecklistLoader {
	return &CoreCapabilityOnboardingChecklistLoader{pool: pool}
}

// CoreOnboardingChecklistItem is a single onboarding step definition.
// Captures the slug, display title, description, the AgentHub resource type
// the user must create/configure, the display order, and flags for whether
// the step is blocking (prevents subsequent steps until complete) and whether
// the platform auto-completes it.
type CoreOnboardingChecklistItem struct {
	Slug         string
	Title        string
	Description  string
	ResourceType string
	StepOrder    int
	IsBlocking   bool
	IsAutomated  bool
}

// LoadCapabilityOnboardingChecklist returns all onboarding checklist rows from
// ah_core.capability_onboarding_checklist WHERE slug = ANY($1), ordered by
// step_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityOnboardingChecklistLoader) LoadCapabilityOnboardingChecklist(ctx context.Context) ([]CoreOnboardingChecklistItem, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, title, description, resource_type, step_order, is_blocking, is_automated
		  FROM ah_core.capability_onboarding_checklist
		 WHERE slug = ANY($1)
		 ORDER BY step_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityOnboardingChecklistSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_onboarding_checklist not accessible, onboarding checklist unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability onboarding checklist: %w", err)
	}
	defer rows.Close()

	var items []CoreOnboardingChecklistItem
	for rows.Next() {
		var item CoreOnboardingChecklistItem
		if err := rows.Scan(
			&item.Slug, &item.Title, &item.Description, &item.ResourceType,
			&item.StepOrder, &item.IsBlocking, &item.IsAutomated,
		); err != nil {
			return nil, fmt.Errorf("core: scan onboarding checklist item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_onboarding_checklist not accessible (post-iter), onboarding checklist unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate onboarding checklist items: %w", err)
	}
	return items, nil
}

// LoadBlockingOnboardingSteps returns only the blocking onboarding checklist
// rows from ah_core.capability_onboarding_checklist WHERE is_blocking = TRUE,
// ordered by step_order. Returns nil, nil when the table is not accessible
// (non-fatal). Blocking steps (steps 1-3) represent the minimum viable
// configuration a tenant must complete before running their first agent.
func (l *CoreCapabilityOnboardingChecklistLoader) LoadBlockingOnboardingSteps(ctx context.Context) ([]CoreOnboardingChecklistItem, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, title, description, resource_type, step_order, is_blocking, is_automated
		  FROM ah_core.capability_onboarding_checklist
		 WHERE is_blocking = TRUE
		 ORDER BY step_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_onboarding_checklist not accessible, blocking onboarding steps unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query blocking onboarding steps: %w", err)
	}
	defer rows.Close()

	var items []CoreOnboardingChecklistItem
	for rows.Next() {
		var item CoreOnboardingChecklistItem
		if err := rows.Scan(
			&item.Slug, &item.Title, &item.Description, &item.ResourceType,
			&item.StepOrder, &item.IsBlocking, &item.IsAutomated,
		); err != nil {
			return nil, fmt.Errorf("core: scan blocking onboarding step: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_onboarding_checklist not accessible (post-iter), blocking onboarding steps unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate blocking onboarding steps: %w", err)
	}
	return items, nil
}

// ============================================================
// Seed catalog constants — migration 000106 (2026-05-11).
// ============================================================

// SeedCapabilityOnboardingChecklistCount is the expected total row count after
// migration 000106. Five onboarding checklist rows — one per guided step from
// zero-config to first successful agent chat session.
const SeedCapabilityOnboardingChecklistCount = 5

// SeedCapabilityOnboardingChecklistSlugs is the canonical closed set of slugs
// seeded by migration 000106, in step_order ascending.
var SeedCapabilityOnboardingChecklistSlugs = []string{
	"onboarding-connect-llm",
	"onboarding-create-agent",
	"onboarding-assign-skill",
	"onboarding-configure-kb",
	"onboarding-test-run",
}

// Slug constants for each onboarding step.

// SeedOnboardingConnectLLMSlug is the slug for step 1: configure an LLM API key.
const SeedOnboardingConnectLLMSlug = "onboarding-connect-llm"

// SeedOnboardingCreateAgentSlug is the slug for step 2: create a capability agent.
const SeedOnboardingCreateAgentSlug = "onboarding-create-agent"

// SeedOnboardingAssignSkillSlug is the slug for step 3: bind a capability skill.
const SeedOnboardingAssignSkillSlug = "onboarding-assign-skill"

// SeedOnboardingConfigureKBSlug is the slug for step 4: create a knowledge base (optional).
const SeedOnboardingConfigureKBSlug = "onboarding-configure-kb"

// SeedOnboardingTestRunSlug is the slug for step 5: run first chat session (optional).
const SeedOnboardingTestRunSlug = "onboarding-test-run"

// Blocking / non-blocking step count constants.

// SeedOnboardingBlockingStepCount is the number of onboarding steps where
// is_blocking=TRUE (steps 1-3: connect-llm, create-agent, assign-skill).
// These are the minimum required to run a capability agent.
const SeedOnboardingBlockingStepCount = 3

// SeedOnboardingNonBlockingStepCount is the number of onboarding steps where
// is_blocking=FALSE (steps 4-5: configure-kb, test-run). These are
// recommended but not required to start the first agent run.
const SeedOnboardingNonBlockingStepCount = 2

// SeedOnboardingResourceTypes is the ordered list of AgentHub resource types
// that each onboarding step requires the tenant to create or configure.
var SeedOnboardingResourceTypes = []string{
	"llm_provider_settings",
	"agent",
	"agent_skill",
	"knowledge_base",
	"chat_session",
}
