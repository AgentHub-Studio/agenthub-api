package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreSubagentToolsetPolicyDefaultTemplate is a platform-managed
// blueprint paired with SUB-005 SubagentToolsetPolicy. Each template
// represents one SubagentToolsetIsolationMode the tenant can pre-pick
// for their subagent definitions, with sample fields populated to
// match the mode's contract.
type CoreSubagentToolsetPolicyDefaultTemplate struct {
	ID                          uuid.UUID
	Slug                        string
	Name                        string
	Description                 string
	TargetIsolationMode         string
	TargetUseCase               string
	SafetyPosture               string
	SampleAllowedToolNamesJSON  string
	SampleBlockedToolNamesJSON  string
	SampleDepthThresholdForAgent int
	SampleCategoryPrefixesJSON  string
	SampleCategorySuffixesJSON  string
	RecommendedForTenantKind    string
	RequiresAdminReview         bool
	IsRecommended               bool
	IsActive                    bool
	SortOrder                   int
}

// SampleAllowedToolNames parses the JSONB array.
func (t CoreSubagentToolsetPolicyDefaultTemplate) SampleAllowedToolNames() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SampleAllowedToolNamesJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_toolset_policy_template %s: invalid sample_allowed_tool_names: %w", t.Slug, err)
	}
	return out, nil
}

// SampleBlockedToolNames parses the JSONB array.
func (t CoreSubagentToolsetPolicyDefaultTemplate) SampleBlockedToolNames() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SampleBlockedToolNamesJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_toolset_policy_template %s: invalid sample_blocked_tool_names: %w", t.Slug, err)
	}
	return out, nil
}

// SampleCategoryPrefixes parses the JSONB array.
func (t CoreSubagentToolsetPolicyDefaultTemplate) SampleCategoryPrefixes() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SampleCategoryPrefixesJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_toolset_policy_template %s: invalid sample_category_prefixes: %w", t.Slug, err)
	}
	return out, nil
}

// SampleCategorySuffixes parses the JSONB array.
func (t CoreSubagentToolsetPolicyDefaultTemplate) SampleCategorySuffixes() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SampleCategorySuffixesJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_toolset_policy_template %s: invalid sample_category_suffixes: %w", t.Slug, err)
	}
	return out, nil
}

// CoreSubagentToolsetPolicyDefaultTemplateLoader loads templates.
type CoreSubagentToolsetPolicyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSubagentToolsetPolicyDefaultTemplateLoader creates the loader.
func NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreSubagentToolsetPolicyDefaultTemplateLoader {
	return &CoreSubagentToolsetPolicyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreSubagentToolsetPolicyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreSubagentToolsetPolicyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_isolation_mode, target_use_case,
		       safety_posture, sample_allowed_tool_names::text,
		       sample_blocked_tool_names::text, sample_depth_threshold_for_agent,
		       sample_category_prefixes::text, sample_category_suffixes::text,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.subagent_toolset_policy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.subagent_toolset_policy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query subagent_toolset_policy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreSubagentToolsetPolicyDefaultTemplate
	for rows.Next() {
		var t CoreSubagentToolsetPolicyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetIsolationMode, &t.TargetUseCase,
			&t.SafetyPosture, &t.SampleAllowedToolNamesJSON,
			&t.SampleBlockedToolNamesJSON, &t.SampleDepthThresholdForAgent,
			&t.SampleCategoryPrefixesJSON, &t.SampleCategorySuffixesJSON,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan subagent_toolset_policy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate subagent_toolset_policy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreSubagentToolsetPolicyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreSubagentToolsetPolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreSubagentToolsetPolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreSubagentToolsetPolicyDefaultTemplate{}, false, nil
}

// LoadByIsolationMode filters by SUB-005 SubagentToolsetIsolationMode.
func (l *CoreSubagentToolsetPolicyDefaultTemplateLoader) LoadByIsolationMode(ctx context.Context, mode string) ([]CoreSubagentToolsetPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentToolsetPolicyDefaultTemplate
	for _, t := range all {
		if t.TargetIsolationMode == mode {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case label.
func (l *CoreSubagentToolsetPolicyDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreSubagentToolsetPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentToolsetPolicyDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreSubagentToolsetPolicyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreSubagentToolsetPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentToolsetPolicyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedSTPDTemplateSlugs is the closed canonical set.
var SeedExpectedSTPDTemplateSlugs = []string{
	"documentation-readonly-allowlist",
	"research-minus-sharp-edges",
	"recursion-safety-depth-filtered",
	"admin-surface-categorical-exclusion",
}

// SeedExpectedSTPDTemplateModes matches SUB-005 SubagentToolsetIsolationMode
// enum byte-for-byte (4 modes).
var SeedExpectedSTPDTemplateModes = []string{
	"explicit_allowlist", "parent_minus_blocklist",
	"depth_filtered", "categorical_exclusion",
}

// SeedExpectedSTPDTemplateUseCases is the closed use-case set.
var SeedExpectedSTPDTemplateUseCases = []string{
	"documentation_generator", "research_assistant",
	"recursion_safety", "sensitive_surface_protection",
}

// SeedExpectedSTPDTemplateSafetyPostures is the closed posture set.
var SeedExpectedSTPDTemplateSafetyPostures = []string{
	"strict", "balanced", "conservative",
}

// SeedExpectedSTPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedSTPDTemplateTenantKinds = []string{"general"}

// SeedRecommendedSTPDTemplateSlugs — all 4 are recommended.
var SeedRecommendedSTPDTemplateSlugs = []string{
	"documentation-readonly-allowlist",
	"research-minus-sharp-edges",
	"recursion-safety-depth-filtered",
	"admin-surface-categorical-exclusion",
}

// SeedAdminReviewSTPDTemplateSlugs — every preset except the routine
// research-minus-sharp-edges requires admin review.
var SeedAdminReviewSTPDTemplateSlugs = []string{
	"documentation-readonly-allowlist",
	"recursion-safety-depth-filtered",
	"admin-surface-categorical-exclusion",
}

// SeedExpectedSTPDTemplateRowCount = 4.
const SeedExpectedSTPDTemplateRowCount = 4
