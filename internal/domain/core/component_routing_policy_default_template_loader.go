package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreComponentRoutingPolicyDefaultTemplate is a platform-managed
// blueprint paired with EXT-005 ComponentRouter. Each row encodes a
// (conflict_policy + use_case) preset tenants pick.
type CoreComponentRoutingPolicyDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	ConflictPolicy           string
	TargetUseCase            string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreComponentRoutingPolicyDefaultTemplateLoader loads routing policy templates.
type CoreComponentRoutingPolicyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreComponentRoutingPolicyDefaultTemplateLoader creates the loader.
func NewCoreComponentRoutingPolicyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreComponentRoutingPolicyDefaultTemplateLoader {
	return &CoreComponentRoutingPolicyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreComponentRoutingPolicyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreComponentRoutingPolicyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, conflict_policy,
		       target_use_case, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.component_routing_policy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.component_routing_policy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query component_routing_policy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreComponentRoutingPolicyDefaultTemplate
	for rows.Next() {
		var t CoreComponentRoutingPolicyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.ConflictPolicy,
			&t.TargetUseCase, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan component_routing_policy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate component_routing_policy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreComponentRoutingPolicyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreComponentRoutingPolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreComponentRoutingPolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreComponentRoutingPolicyDefaultTemplate{}, false, nil
}

// LoadByPolicy filters by EXT-005 conflict policy label.
func (l *CoreComponentRoutingPolicyDefaultTemplateLoader) LoadByPolicy(ctx context.Context, policy string) ([]CoreComponentRoutingPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreComponentRoutingPolicyDefaultTemplate
	for _, t := range all {
		if t.ConflictPolicy == policy {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreComponentRoutingPolicyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreComponentRoutingPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreComponentRoutingPolicyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedCRPDTemplateSlugs is the closed canonical set.
var SeedExpectedCRPDTemplateSlugs = []string{
	"stable-incumbent",
	"optimistic-latest",
	"strict-pinned",
	"fail-fast",
	"dev-debug-incumbent",
}

// SeedExpectedCRPDTemplatePolicies matches EXT-005 RoutingConflictPolicy enum.
var SeedExpectedCRPDTemplatePolicies = []string{
	"first_install_wins", "latest_install_wins",
	"require_explicit_pin", "error_on_conflict",
}

// SeedExpectedCRPDTemplateUseCases is the closed use-case set.
var SeedExpectedCRPDTemplateUseCases = []string{
	"general", "regulated", "staging",
}

// SeedExpectedCRPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedCRPDTemplateTenantKinds = []string{
	"general", "regulated", "dev_local",
}

// SeedRecommendedCRPDTemplateSlugs lists the safe one-click subset.
var SeedRecommendedCRPDTemplateSlugs = []string{
	"stable-incumbent",
	"optimistic-latest",
	"strict-pinned",
	"fail-fast",
}

// SeedAdminReviewCRPDTemplateSlugs is the admin-review set.
var SeedAdminReviewCRPDTemplateSlugs = []string{
	"strict-pinned",
}

// SeedExpectedCRPDTemplateRowCount = 5.
const SeedExpectedCRPDTemplateRowCount = 5
