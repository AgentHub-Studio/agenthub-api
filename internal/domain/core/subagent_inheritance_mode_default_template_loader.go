package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreSubagentInheritanceModeDefaultTemplate is a platform-managed
// blueprint paired with SUB-006 ResolveSubagentPermissions. Each
// template represents one PermissionInheritanceMode the tenant can
// pre-pick for their subagent definitions.
type CoreSubagentInheritanceModeDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetInheritanceMode    string
	TargetUseCase            string
	RiskPosture              string
	ExpectedAuditSignalsJSON string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// ExpectedAuditSignals parses the JSONB array of audit fields the
// caller should monitor when this mode is in effect.
func (t CoreSubagentInheritanceModeDefaultTemplate) ExpectedAuditSignals() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.ExpectedAuditSignalsJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_inheritance_mode_template %s: invalid expected_audit_signals: %w", t.Slug, err)
	}
	return out, nil
}

// CoreSubagentInheritanceModeDefaultTemplateLoader loads templates.
type CoreSubagentInheritanceModeDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSubagentInheritanceModeDefaultTemplateLoader creates the loader.
func NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool *pgxpool.Pool) *CoreSubagentInheritanceModeDefaultTemplateLoader {
	return &CoreSubagentInheritanceModeDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreSubagentInheritanceModeDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreSubagentInheritanceModeDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_inheritance_mode, target_use_case,
		       risk_posture, expected_audit_signals::text,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.subagent_inheritance_mode_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.subagent_inheritance_mode_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query subagent_inheritance_mode_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreSubagentInheritanceModeDefaultTemplate
	for rows.Next() {
		var t CoreSubagentInheritanceModeDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetInheritanceMode, &t.TargetUseCase,
			&t.RiskPosture, &t.ExpectedAuditSignalsJSON,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan subagent_inheritance_mode_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate subagent_inheritance_mode_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreSubagentInheritanceModeDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreSubagentInheritanceModeDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreSubagentInheritanceModeDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreSubagentInheritanceModeDefaultTemplate{}, false, nil
}

// LoadByInheritanceMode filters by SUB-006 PermissionInheritanceMode.
func (l *CoreSubagentInheritanceModeDefaultTemplateLoader) LoadByInheritanceMode(ctx context.Context, mode string) ([]CoreSubagentInheritanceModeDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentInheritanceModeDefaultTemplate
	for _, t := range all {
		if t.TargetInheritanceMode == mode {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByRiskPosture filters by posture label.
func (l *CoreSubagentInheritanceModeDefaultTemplateLoader) LoadByRiskPosture(ctx context.Context, posture string) ([]CoreSubagentInheritanceModeDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentInheritanceModeDefaultTemplate
	for _, t := range all {
		if t.RiskPosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreSubagentInheritanceModeDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreSubagentInheritanceModeDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentInheritanceModeDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedSIMDTemplateSlugs is the closed canonical set.
var SeedExpectedSIMDTemplateSlugs = []string{
	"extend-parent-rights",
	"sandboxed-worker",
	"isolated-decoupled",
	"audit-strict-intersect",
}

// SeedExpectedSIMDTemplateModes matches SUB-006 PermissionInheritanceMode
// enum byte-for-byte (4 modes).
var SeedExpectedSIMDTemplateModes = []string{
	"inherit_all", "inherit_strict_only",
	"override_replace", "merge_intersect",
}

// SeedExpectedSIMDTemplateUseCases is the closed use-case set.
var SeedExpectedSIMDTemplateUseCases = []string{
	"helper_extension", "narrow_utility",
	"explicit_isolation", "compliance_audit",
}

// SeedExpectedSIMDTemplateRiskPostures is the closed posture set.
var SeedExpectedSIMDTemplateRiskPostures = []string{
	"balanced", "conservative", "strict", "permissive",
}

// SeedExpectedSIMDTemplateAuditSignals enumerates the audit fields the
// SUB-006 resolver populates (must align with
// SubagentPermissionResolution struct exposed audit fields).
var SeedExpectedSIMDTemplateAuditSignals = []string{
	"AddedAllows", "AddedDenies", "DroppedAllows", "ReasonSummary",
}

// SeedExpectedSIMDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedSIMDTemplateTenantKinds = []string{"general"}

// SeedRecommendedSIMDTemplateSlugs — all 4 are recommended.
var SeedRecommendedSIMDTemplateSlugs = []string{
	"extend-parent-rights",
	"sandboxed-worker",
	"isolated-decoupled",
	"audit-strict-intersect",
}

// SeedAdminReviewSIMDTemplateSlugs — every preset except the routine
// extend-parent-rights requires admin review (others materially change
// permission composition).
var SeedAdminReviewSIMDTemplateSlugs = []string{
	"sandboxed-worker",
	"isolated-decoupled",
	"audit-strict-intersect",
}

// SeedExpectedSIMDTemplateRowCount = 4.
const SeedExpectedSIMDTemplateRowCount = 4
