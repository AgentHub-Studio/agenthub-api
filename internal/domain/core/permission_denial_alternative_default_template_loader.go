package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePermissionDenialAlternativeDefaultTemplate is a platform-managed
// blueprint paired with PERM-006 PermissionDeniedFeedback. Each
// template captures: "when tool X is denied for reason Y, suggest
// alternatives Z" so the LLM does not have to discover pivots.
type CorePermissionDenialAlternativeDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetReason             string
	TargetRetryHint          string
	DeniedToolName           string
	AlternativeToolNamesJSON string
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// AlternativeToolNames parses the JSONB array.
func (t CorePermissionDenialAlternativeDefaultTemplate) AlternativeToolNames() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.AlternativeToolNamesJSON), &out); err != nil {
		return nil, fmt.Errorf("permission_denial_alternative_template %s: invalid alternative_tool_names: %w", t.Slug, err)
	}
	return out, nil
}

// CorePermissionDenialAlternativeDefaultTemplateLoader loads templates.
type CorePermissionDenialAlternativeDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePermissionDenialAlternativeDefaultTemplateLoader creates the
// loader.
func NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool *pgxpool.Pool) *CorePermissionDenialAlternativeDefaultTemplateLoader {
	return &CorePermissionDenialAlternativeDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePermissionDenialAlternativeDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePermissionDenialAlternativeDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_reason, target_retry_hint,
		       denied_tool_name, alternative_tool_names::text,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.permission_denial_alternative_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.permission_denial_alternative_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query permission_denial_alternative_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePermissionDenialAlternativeDefaultTemplate
	for rows.Next() {
		var t CorePermissionDenialAlternativeDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetReason, &t.TargetRetryHint,
			&t.DeniedToolName, &t.AlternativeToolNamesJSON,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan permission_denial_alternative_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate permission_denial_alternative_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePermissionDenialAlternativeDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePermissionDenialAlternativeDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePermissionDenialAlternativeDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePermissionDenialAlternativeDefaultTemplate{}, false, nil
}

// LoadByReason filters by PERM-006 PermissionDenialReason label.
func (l *CorePermissionDenialAlternativeDefaultTemplateLoader) LoadByReason(ctx context.Context, reason string) ([]CorePermissionDenialAlternativeDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionDenialAlternativeDefaultTemplate
	for _, t := range all {
		if t.TargetReason == reason {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByRetryHint filters by PERM-006 PermissionRetryHint label.
func (l *CorePermissionDenialAlternativeDefaultTemplateLoader) LoadByRetryHint(ctx context.Context, hint string) ([]CorePermissionDenialAlternativeDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionDenialAlternativeDefaultTemplate
	for _, t := range all {
		if t.TargetRetryHint == hint {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByDeniedTool filters by denied tool name.
func (l *CorePermissionDenialAlternativeDefaultTemplateLoader) LoadByDeniedTool(ctx context.Context, toolName string) ([]CorePermissionDenialAlternativeDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionDenialAlternativeDefaultTemplate
	for _, t := range all {
		if t.DeniedToolName == toolName {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePermissionDenialAlternativeDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePermissionDenialAlternativeDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePermissionDenialAlternativeDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPDADTemplateSlugs is the closed canonical set.
var SeedExpectedPDADTemplateSlugs = []string{
	"bash-to-sandboxed-shell",
	"execute-sql-to-document-search",
	"write-to-edit-fallback",
	"http-fetch-rate-limited-wait",
	"mcp-tool-prefilter-drop-pivot",
	"dontask-mode-confirm-required",
}

// SeedExpectedPDADTemplateReasons matches PERM-006 PermissionDenialReason
// enum byte-for-byte (6 reasons — at least one example for 4 of them;
// sandbox_violation + hook_override are exercised by other templates).
var SeedExpectedPDADTemplateReasons = []string{
	"rule_match", "hook_override", "prefilter_drop",
	"mode_block", "rate_limit",
}

// SeedExpectedPDADTemplateRetryHints matches PERM-006
// PermissionRetryHint enum byte-for-byte.
var SeedExpectedPDADTemplateRetryHints = []string{
	"suggest_alternative_tool", "wait_and_retry",
	"request_user_confirmation",
}

// SeedExpectedPDADTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedPDADTemplateTenantKinds = []string{"general"}

// SeedRecommendedPDADTemplateSlugs — all 6 are recommended.
var SeedRecommendedPDADTemplateSlugs = []string{
	"bash-to-sandboxed-shell",
	"execute-sql-to-document-search",
	"write-to-edit-fallback",
	"http-fetch-rate-limited-wait",
	"mcp-tool-prefilter-drop-pivot",
	"dontask-mode-confirm-required",
}

// SeedAdminReviewPDADTemplateSlugs — only dont_ask mode template
// requires review (mode change has session-wide impact).
var SeedAdminReviewPDADTemplateSlugs = []string{
	"dontask-mode-confirm-required",
}

// SeedExpectedPDADTemplateRowCount = 6.
const SeedExpectedPDADTemplateRowCount = 6
