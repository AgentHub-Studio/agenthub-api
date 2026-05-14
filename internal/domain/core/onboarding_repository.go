package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// OnboardingSignals captures the per-tenant completion state behind each
// onboarding checklist step. Every field is derived from a cheap EXISTS query
// against the calling tenant's own schema.
type OnboardingSignals struct {
	// LLMConfigured is true when any LLM provider key/base URL is set in settings.
	LLMConfigured bool
	// HasUserAgent is true when the tenant has an agent other than the
	// auto-seeded default assistant (slug agenthub-assistant).
	HasUserAgent bool
	// HasSkillBound is true when a skill is bound to an agent other than the
	// auto-seeded default assistant. The built-in assistant ships with seeded
	// skill bindings, so a bare agent_skill existence check would always be
	// true — the onboarding step is about the user binding a skill to their
	// own agent.
	HasSkillBound bool
	// HasKnowledgeBase is true when at least one knowledge base exists.
	HasKnowledgeBase bool
	// HasCompletedRun is true when at least one chat run reached 'completed'.
	HasCompletedRun bool
	// Dismissed is true when settings.onboarding.completed was explicitly set true.
	Dismissed bool
}

// OnboardingStatusRepository reads onboarding completion signals from a tenant
// schema. It is read-only and tenant-scoped via the request context.
type OnboardingStatusRepository struct {
	pool *pgxpool.Pool
}

// NewOnboardingStatusRepository creates an OnboardingStatusRepository backed by pool.
func NewOnboardingStatusRepository(pool *pgxpool.Pool) *OnboardingStatusRepository {
	return &OnboardingStatusRepository{pool: pool}
}

// LoadSignals runs the five completion-signal checks plus the manual-dismiss
// flag in a single round-trip against the tenant's schema. The jsonb value::text
// comparisons treat an empty/absent string ('""'), a jsonb null and an empty
// column the same way — "not configured".
func (r *OnboardingStatusRepository) LoadSignals(ctx context.Context) (OnboardingSignals, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return OnboardingSignals{}, err
	}
	defer release()

	var s OnboardingSignals
	err = conn.QueryRow(ctx, `
		SELECT
		    EXISTS (
		        SELECT 1 FROM settings
		        WHERE key IN ('openrouter.apiKey', 'openai.apiKey', 'claude.apiKey', 'ollama.baseUrl')
		          AND value::text NOT IN ('""', 'null', '')
		    ),
		    EXISTS (SELECT 1 FROM agent WHERE slug <> 'agenthub-assistant'),
		    EXISTS (
		        SELECT 1 FROM agent_skill ash
		        JOIN agent a ON a.id = ash.agent_id
		        WHERE a.slug <> 'agenthub-assistant'
		    ),
		    EXISTS (SELECT 1 FROM knowledge_base),
		    EXISTS (SELECT 1 FROM chat_run WHERE status = 'completed'),
		    COALESCE((SELECT value::text = 'true' FROM settings WHERE key = 'onboarding.completed'), FALSE)
	`).Scan(
		&s.LLMConfigured,
		&s.HasUserAgent,
		&s.HasSkillBound,
		&s.HasKnowledgeBase,
		&s.HasCompletedRun,
		&s.Dismissed,
	)
	if err != nil {
		return OnboardingSignals{}, fmt.Errorf("core: load onboarding signals: %w", err)
	}
	return s, nil
}
