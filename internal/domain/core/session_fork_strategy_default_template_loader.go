package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreSessionForkStrategyDefaultTemplate is a platform-managed
// blueprint paired with PERSIST-005a SessionForkStrategy. Each template
// captures cost × safety × compaction-tolerance trade-offs for one
// fork strategy.
type CoreSessionForkStrategyDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetStrategy           string
	TargetUseCase            string
	SafetyPosture            string
	ComputesTranscriptHash   bool
	AllowsForkPastCompaction bool
	TypicalStorageOverhead   string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreSessionForkStrategyDefaultTemplateLoader loads templates.
type CoreSessionForkStrategyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSessionForkStrategyDefaultTemplateLoader creates the loader.
func NewCoreSessionForkStrategyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreSessionForkStrategyDefaultTemplateLoader {
	return &CoreSessionForkStrategyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreSessionForkStrategyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreSessionForkStrategyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_strategy, target_use_case,
		       safety_posture, computes_transcript_hash, allows_fork_past_compaction,
		       typical_storage_overhead, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.session_fork_strategy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.session_fork_strategy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query session_fork_strategy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreSessionForkStrategyDefaultTemplate
	for rows.Next() {
		var t CoreSessionForkStrategyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetStrategy, &t.TargetUseCase,
			&t.SafetyPosture, &t.ComputesTranscriptHash, &t.AllowsForkPastCompaction,
			&t.TypicalStorageOverhead, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan session_fork_strategy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate session_fork_strategy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreSessionForkStrategyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreSessionForkStrategyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreSessionForkStrategyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreSessionForkStrategyDefaultTemplate{}, false, nil
}

// LoadByStrategy filters by PERSIST-005a SessionForkStrategy.
func (l *CoreSessionForkStrategyDefaultTemplateLoader) LoadByStrategy(ctx context.Context, strategy string) ([]CoreSessionForkStrategyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSessionForkStrategyDefaultTemplate
	for _, t := range all {
		if t.TargetStrategy == strategy {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case.
func (l *CoreSessionForkStrategyDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreSessionForkStrategyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSessionForkStrategyDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreSessionForkStrategyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreSessionForkStrategyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSessionForkStrategyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedSFSDTemplateSlugs is the closed canonical set.
var SeedExpectedSFSDTemplateSlugs = []string{
	"full-copy-explore",
	"branch-pointer-cheap",
	"snapshot-isolated-compliance",
}

// SeedExpectedSFSDTemplateStrategies matches PERSIST-005a
// SessionForkStrategy enum byte-for-byte (3 strategies).
var SeedExpectedSFSDTemplateStrategies = []string{
	"full_copy", "branch_pointer", "snapshot_isolated",
}

// SeedExpectedSFSDTemplateUseCases is the closed use-case set.
var SeedExpectedSFSDTemplateUseCases = []string{
	"routine_branch_exploration", "storage_optimized_branch", "compliance_branch",
}

// SeedExpectedSFSDTemplateSafetyPostures is the closed posture set.
var SeedExpectedSFSDTemplateSafetyPostures = []string{
	"balanced", "permissive", "strict",
}

// SeedExpectedSFSDTemplateStorageOverheads is the closed overhead set.
var SeedExpectedSFSDTemplateStorageOverheads = []string{
	"low", "medium", "high",
}

// SeedExpectedSFSDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedSFSDTemplateTenantKinds = []string{"general"}

// SeedRecommendedSFSDTemplateSlugs — all 3 are recommended.
var SeedRecommendedSFSDTemplateSlugs = []string{
	"full-copy-explore",
	"branch-pointer-cheap",
	"snapshot-isolated-compliance",
}

// SeedAdminReviewSFSDTemplateSlugs — only the compliance strategy
// requires admin review (it crosses compact boundary so semantics are
// non-trivial).
var SeedAdminReviewSFSDTemplateSlugs = []string{
	"snapshot-isolated-compliance",
}

// SeedExpectedSFSDTemplateRowCount = 3.
const SeedExpectedSFSDTemplateRowCount = 3
