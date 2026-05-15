package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityFeatureFlagLoader loads per-capability feature toggle settings
// from ah_core.capability_feature_flag. These 6 rows (seeded by migration
// 000103) provide feature flags adapted from Claude Code Appendix A.2 Table 8
// (conditional tool availability) for the AgentHub capability layer:
//
//   - capability-citations               (research,      default_enabled=true)
//   - capability-task-tracking           (planning,      default_enabled=true)
//   - capability-kb-indexing             (research,      default_enabled=true)
//   - capability-subagent-delegation     (orchestration, default_enabled=false)
//   - capability-doc-citations           (analysis,      default_enabled=true)
//   - capability-progressive-summarization (memory,      default_enabled=true)
//
// Non-fatal when the ah_core schema or the capability_feature_flag table
// is missing — supports fresh deployments where migration 000103 has not yet run.
type CoreCapabilityFeatureFlagLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityFeatureFlagLoader creates a
// CoreCapabilityFeatureFlagLoader backed by pool.
func NewCoreCapabilityFeatureFlagLoader(pool *pgxpool.Pool) *CoreCapabilityFeatureFlagLoader {
	return &CoreCapabilityFeatureFlagLoader{pool: pool}
}

// CoreCapabilityFeatureFlag is a single capability-layer feature toggle.
// Captures the slug, display name, description, whether it is enabled by
// default, the category grouping, and its display sort position.
type CoreCapabilityFeatureFlag struct {
	Slug           string
	Name           string
	Description    string
	DefaultEnabled bool
	Category       string
	SortOrder      int
}

// LoadCapabilityFeatureFlags returns all capability feature flag rows from
// ah_core.capability_feature_flag WHERE slug = ANY($1), ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityFeatureFlagLoader) LoadCapabilityFeatureFlags(ctx context.Context) ([]CoreCapabilityFeatureFlag, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, name, COALESCE(description, ''), default_enabled, category, sort_order
		  FROM ah_core.capability_feature_flag
		 WHERE slug = ANY($1)
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityFeatureFlagSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_feature_flag not accessible, capability feature flags unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability feature flags: %w", err)
	}
	defer rows.Close()

	var flags []CoreCapabilityFeatureFlag
	for rows.Next() {
		var f CoreCapabilityFeatureFlag
		if err := rows.Scan(
			&f.Slug, &f.Name, &f.Description, &f.DefaultEnabled, &f.Category, &f.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability feature flag: %w", err)
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_feature_flag not accessible (post-iter), capability feature flags unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability feature flags: %w", err)
	}
	return flags, nil
}

// LoadEnabledCapabilityFeatureFlags returns all capability feature flag rows
// where default_enabled = TRUE from ah_core.capability_feature_flag, ordered
// by sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityFeatureFlagLoader) LoadEnabledCapabilityFeatureFlags(ctx context.Context) ([]CoreCapabilityFeatureFlag, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, name, COALESCE(description, ''), default_enabled, category, sort_order
		  FROM ah_core.capability_feature_flag
		 WHERE default_enabled = TRUE
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_feature_flag not accessible, enabled capability feature flags unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query enabled capability feature flags: %w", err)
	}
	defer rows.Close()

	var flags []CoreCapabilityFeatureFlag
	for rows.Next() {
		var f CoreCapabilityFeatureFlag
		if err := rows.Scan(
			&f.Slug, &f.Name, &f.Description, &f.DefaultEnabled, &f.Category, &f.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan enabled capability feature flag: %w", err)
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_feature_flag not accessible (post-iter), enabled capability feature flags unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate enabled capability feature flags: %w", err)
	}
	return flags, nil
}

// ============================================================
// Seed catalog constants — migration 000103 (2026-05-11).
// ============================================================

// SeedCapabilityFeatureFlagCount is the expected total row count after
// migration 000103. Six feature flag rows across five capability categories
// (research ×2, planning ×1, orchestration ×1, analysis ×1, memory ×1).
const SeedCapabilityFeatureFlagCount = 6

// SeedCapabilityFeatureFlagSlugs is the canonical closed set of feature flag
// slugs seeded by migration 000103.
var SeedCapabilityFeatureFlagSlugs = []string{
	"capability-citations",
	"capability-task-tracking",
	"capability-kb-indexing",
	"capability-subagent-delegation",
	"capability-doc-citations",
	"capability-progressive-summarization",
}

// Individual slug constants for each seeded feature flag.

// SeedCitationsFeatureFlagSlug is the slug for the Web Citation Tracking flag
// (research category, default_enabled=true).
const SeedCitationsFeatureFlagSlug = "capability-citations"

// SeedTaskTrackingFeatureFlagSlug is the slug for the Task Progress Tracking
// flag (planning category, default_enabled=true).
const SeedTaskTrackingFeatureFlagSlug = "capability-task-tracking"

// SeedKBIndexingFeatureFlagSlug is the slug for the Knowledge Base
// Auto-Indexing flag (research category, default_enabled=true).
const SeedKBIndexingFeatureFlagSlug = "capability-kb-indexing"

// SeedSubagentDelegationFeatureFlagSlug is the slug for the Subagent
// Delegation flag (orchestration category, default_enabled=false).
// This is the only flag disabled by default — spawning subagents requires
// explicit opt-in to avoid unintended cost and recursion in new tenants.
const SeedSubagentDelegationFeatureFlagSlug = "capability-subagent-delegation"

// SeedDocCitationsFeatureFlagSlug is the slug for the Document Citation
// Tracking flag (analysis category, default_enabled=true).
const SeedDocCitationsFeatureFlagSlug = "capability-doc-citations"

// SeedProgressiveSummarizationFeatureFlagSlug is the slug for the Progressive
// Summarization flag (memory category, default_enabled=true).
const SeedProgressiveSummarizationFeatureFlagSlug = "capability-progressive-summarization"

// SeedDisabledByDefaultFeatureFlagSlug is the slug of the single feature flag
// that is disabled by default in migration 000103 (subagent-delegation).
const SeedDisabledByDefaultFeatureFlagSlug = SeedSubagentDelegationFeatureFlagSlug

// Category constants for the five feature flag groups.

// SeedFeatureFlagCategoryResearch is the category for web citation tracking
// and knowledge base auto-indexing flags.
const SeedFeatureFlagCategoryResearch = "research"

// SeedFeatureFlagCategoryPlanning is the category for task progress tracking.
const SeedFeatureFlagCategoryPlanning = "planning"

// SeedFeatureFlagCategoryAnalysis is the category for document citation tracking.
const SeedFeatureFlagCategoryAnalysis = "analysis"

// SeedFeatureFlagCategoryOrchestration is the category for subagent delegation.
const SeedFeatureFlagCategoryOrchestration = "orchestration"

// SeedFeatureFlagCategoryMemory is the category for progressive summarization.
const SeedFeatureFlagCategoryMemory = "memory"
