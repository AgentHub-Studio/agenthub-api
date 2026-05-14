package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePolicyEngineTemplate represents a platform-managed policy template
// catalog entry. Tenants opt-in by creating a policy from a template;
// values become the policy's initial config.
//
// Inspired by PDF arXiv:2604.14228v1 §11 (governance — pluggable policy
// backends) + GOV-002 PolicyEngine.
type CorePolicyEngineTemplate struct {
	ID                     uuid.UUID
	Slug                   string
	DisplayName            string
	Description            string
	ComplianceProfile      string // gdpr/hipaa/sox/pci_dss/soc2/iso27001/generic_safety
	EngineKind             string // static_deny/limits_backed/chained
	DenyTools              string // comma-separated
	RequireApprovalTools   string // comma-separated
	Obligations            string // comma-separated
	RequiresAdminReview    bool
	IsRecommended          bool
	IsActive               bool
	SortOrder              int
}

// DenyToolsList parses DenyTools.
func (t CorePolicyEngineTemplate) DenyToolsList() []string {
	return parseCommaList(t.DenyTools)
}

// RequireApprovalToolsList parses RequireApprovalTools.
func (t CorePolicyEngineTemplate) RequireApprovalToolsList() []string {
	return parseCommaList(t.RequireApprovalTools)
}

// ObligationsList parses Obligations.
func (t CorePolicyEngineTemplate) ObligationsList() []string {
	return parseCommaList(t.Obligations)
}

func parseCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// CorePolicyEngineTemplateLoader loads policy templates.
type CorePolicyEngineTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePolicyEngineTemplateLoader creates a CorePolicyEngineTemplateLoader.
func NewCorePolicyEngineTemplateLoader(pool *pgxpool.Pool) *CorePolicyEngineTemplateLoader {
	return &CorePolicyEngineTemplateLoader{pool: pool}
}

// LoadAll returns all active templates ordered by sort_order then slug.
func (l *CorePolicyEngineTemplateLoader) LoadAll(ctx context.Context) ([]CorePolicyEngineTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description,
		       compliance_profile, engine_kind,
		       deny_tools, require_approval_tools, obligations,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.policy_engine_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.policy_engine_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query policy_engine_template: %w", err)
	}
	defer rows.Close()

	var templates []CorePolicyEngineTemplate
	for rows.Next() {
		var t CorePolicyEngineTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.DisplayName, &t.Description,
			&t.ComplianceProfile, &t.EngineKind,
			&t.DenyTools, &t.RequireApprovalTools, &t.Obligations,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan policy_engine_template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate policy_engine_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CorePolicyEngineTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePolicyEngineTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePolicyEngineTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePolicyEngineTemplate{}, false, nil
}

// LoadByComplianceProfile returns templates for a specific profile.
func (l *CorePolicyEngineTemplateLoader) LoadByComplianceProfile(ctx context.Context, profile string) ([]CorePolicyEngineTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePolicyEngineTemplate
	for _, t := range all {
		if t.ComplianceProfile == profile {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended.
func (l *CorePolicyEngineTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePolicyEngineTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePolicyEngineTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPolicyTemplateSlugs is the canonical list of slugs.
var SeedExpectedPolicyTemplateSlugs = []string{
	"generic-safety",
	"gdpr-strict",
	"hipaa-strict",
	"sox-financial",
	"pci-dss-cardholder",
	"soc2-baseline",
	"iso27001-info-security",
}

// SeedExpectedPolicyTemplateComplianceProfiles is the closed set.
var SeedExpectedPolicyTemplateComplianceProfiles = []string{
	"generic_safety",
	"gdpr",
	"hipaa",
	"sox",
	"pci_dss",
	"soc2",
	"iso27001",
}

// SeedExpectedPolicyTemplateEngineKinds is the closed set.
var SeedExpectedPolicyTemplateEngineKinds = []string{
	"static_deny",
	"limits_backed",
	"chained",
}

// SeedRecommendedPolicyTemplateSlugs is the curated subset (safe defaults).
var SeedRecommendedPolicyTemplateSlugs = []string{
	"generic-safety",
	"soc2-baseline",
}

// SeedAdminReviewRequiredPolicyTemplateSlugs lists templates requiring
// admin approval (regulated profiles).
var SeedAdminReviewRequiredPolicyTemplateSlugs = []string{
	"gdpr-strict",
	"hipaa-strict",
	"sox-financial",
	"pci-dss-cardholder",
	"iso27001-info-security",
}
