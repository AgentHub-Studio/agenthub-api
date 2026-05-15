package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreAutoMemoryClassifierDefaultTemplate is a platform-managed
// blueprint paired with CTX-006 AutoMemoryConfig. Each row encodes a
// posture fresh tenants pick instead of inventing thresholds.
type CoreAutoMemoryClassifierDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	MinConfidence            float64
	MaxPerTurn               int
	AdminBlockedKeys         string
	TargetPosture            string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// AdminBlockedKeysList parses comma-separated block list.
func (t CoreAutoMemoryClassifierDefaultTemplate) AdminBlockedKeysList() []string {
	if strings.TrimSpace(t.AdminBlockedKeys) == "" {
		return nil
	}
	parts := strings.Split(t.AdminBlockedKeys, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CoreAutoMemoryClassifierDefaultTemplateLoader loads classifier defaults.
type CoreAutoMemoryClassifierDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreAutoMemoryClassifierDefaultTemplateLoader creates the loader.
func NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool *pgxpool.Pool) *CoreAutoMemoryClassifierDefaultTemplateLoader {
	return &CoreAutoMemoryClassifierDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreAutoMemoryClassifierDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreAutoMemoryClassifierDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, min_confidence, max_per_turn,
		       admin_blocked_keys, target_posture, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.auto_memory_classifier_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.auto_memory_classifier_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query auto_memory_classifier_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreAutoMemoryClassifierDefaultTemplate
	for rows.Next() {
		var t CoreAutoMemoryClassifierDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.MinConfidence, &t.MaxPerTurn,
			&t.AdminBlockedKeys, &t.TargetPosture, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan auto_memory_classifier_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate auto_memory_classifier_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreAutoMemoryClassifierDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreAutoMemoryClassifierDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreAutoMemoryClassifierDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreAutoMemoryClassifierDefaultTemplate{}, false, nil
}

// LoadByPosture filters by target_posture.
func (l *CoreAutoMemoryClassifierDefaultTemplateLoader) LoadByPosture(ctx context.Context, posture string) ([]CoreAutoMemoryClassifierDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreAutoMemoryClassifierDefaultTemplate
	for _, t := range all {
		if t.TargetPosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreAutoMemoryClassifierDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreAutoMemoryClassifierDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreAutoMemoryClassifierDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedAMCDTemplateSlugs is the closed canonical set.
var SeedExpectedAMCDTemplateSlugs = []string{
	"balanced-default",
	"strict-conservative",
	"lenient-exploration",
	"privacy-first",
	"pii-strict",
	"dev-debug",
}

// SeedExpectedAMCDTemplatePostures is the closed posture set.
var SeedExpectedAMCDTemplatePostures = []string{
	"balanced", "strict", "lenient",
	"privacy_first", "pii_strict", "dev_debug",
}

// SeedExpectedAMCDTemplateTenantKinds is the closed audience set.
var SeedExpectedAMCDTemplateTenantKinds = []string{
	"general", "regulated", "dev_local",
}

// SeedRecommendedAMCDTemplateSlugs is the safe subset (excludes dev-debug
// which is opt-in-for-dev-only).
var SeedRecommendedAMCDTemplateSlugs = []string{
	"balanced-default",
	"strict-conservative",
	"lenient-exploration",
	"privacy-first",
	"pii-strict",
}

// SeedAdminReviewAMCDTemplateSlugs lists templates requiring admin
// review (privacy-first + pii-strict have org-wide privacy implications).
var SeedAdminReviewAMCDTemplateSlugs = []string{
	"privacy-first",
	"pii-strict",
}

// SeedExpectedAMCDTemplateRowCount = 6.
const SeedExpectedAMCDTemplateRowCount = 6
