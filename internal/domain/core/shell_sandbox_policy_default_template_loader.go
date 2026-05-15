package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreShellSandboxPolicyDefaultTemplate is a platform-managed blueprint
// paired with PERM-008 ShellSandbox. Each template ships a complete
// policy snapshot (AllowedPathRoots, BlockedCommands, BlockedArgPatterns,
// MaxRuntimeSecs, MaxOutputBytes, AllowNetwork) for a known safety
// posture so tenants do not invent their own.
type CoreShellSandboxPolicyDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetSafetyPosture      string
	TargetUseCase            string
	AllowedPathRootsJSON     string
	BlockedCommandsJSON      string
	BlockedArgPatternsJSON   string
	MaxRuntimeSecs           int
	MaxOutputBytes           int
	AllowNetwork             bool
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// AllowedPathRoots parses the JSONB array.
func (t CoreShellSandboxPolicyDefaultTemplate) AllowedPathRoots() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.AllowedPathRootsJSON), &out); err != nil {
		return nil, fmt.Errorf("shell_sandbox_policy_template %s: invalid allowed_path_roots: %w", t.Slug, err)
	}
	return out, nil
}

// BlockedCommands parses the JSONB array.
func (t CoreShellSandboxPolicyDefaultTemplate) BlockedCommands() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.BlockedCommandsJSON), &out); err != nil {
		return nil, fmt.Errorf("shell_sandbox_policy_template %s: invalid blocked_commands: %w", t.Slug, err)
	}
	return out, nil
}

// BlockedArgPatterns parses the JSONB array.
func (t CoreShellSandboxPolicyDefaultTemplate) BlockedArgPatterns() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.BlockedArgPatternsJSON), &out); err != nil {
		return nil, fmt.Errorf("shell_sandbox_policy_template %s: invalid blocked_arg_patterns: %w", t.Slug, err)
	}
	return out, nil
}

// CoreShellSandboxPolicyDefaultTemplateLoader loads templates.
type CoreShellSandboxPolicyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreShellSandboxPolicyDefaultTemplateLoader creates the loader.
func NewCoreShellSandboxPolicyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreShellSandboxPolicyDefaultTemplateLoader {
	return &CoreShellSandboxPolicyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreShellSandboxPolicyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreShellSandboxPolicyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_safety_posture, target_use_case,
		       allowed_path_roots::text, blocked_commands::text,
		       blocked_arg_patterns::text, max_runtime_secs, max_output_bytes,
		       allow_network, recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.shell_sandbox_policy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.shell_sandbox_policy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query shell_sandbox_policy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreShellSandboxPolicyDefaultTemplate
	for rows.Next() {
		var t CoreShellSandboxPolicyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetSafetyPosture, &t.TargetUseCase,
			&t.AllowedPathRootsJSON, &t.BlockedCommandsJSON,
			&t.BlockedArgPatternsJSON, &t.MaxRuntimeSecs, &t.MaxOutputBytes,
			&t.AllowNetwork, &t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan shell_sandbox_policy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate shell_sandbox_policy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreShellSandboxPolicyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreShellSandboxPolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreShellSandboxPolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreShellSandboxPolicyDefaultTemplate{}, false, nil
}

// LoadBySafetyPosture filters by posture label.
func (l *CoreShellSandboxPolicyDefaultTemplateLoader) LoadBySafetyPosture(ctx context.Context, posture string) ([]CoreShellSandboxPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreShellSandboxPolicyDefaultTemplate
	for _, t := range all {
		if t.TargetSafetyPosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case label.
func (l *CoreShellSandboxPolicyDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreShellSandboxPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreShellSandboxPolicyDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreShellSandboxPolicyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreShellSandboxPolicyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreShellSandboxPolicyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedSSPDTemplateSlugs is the closed canonical set.
var SeedExpectedSSPDTemplateSlugs = []string{
	"locked-down",
	"web-safe-default",
	"dev-workstation",
	"cicd-runner",
	"incident-response-readonly",
}

// SeedExpectedSSPDTemplateSafetyPostures is the closed posture set.
var SeedExpectedSSPDTemplateSafetyPostures = []string{
	"strict", "balanced", "progressive", "permissive", "conservative",
}

// SeedExpectedSSPDTemplateUseCases is the closed use-case set.
var SeedExpectedSSPDTemplateUseCases = []string{
	"audit_session", "standard_chat", "engineering",
	"cicd_pipeline", "incident_response",
}

// SeedExpectedSSPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedSSPDTemplateTenantKinds = []string{"general"}

// SeedRecommendedSSPDTemplateSlugs — all 5 are recommended.
var SeedRecommendedSSPDTemplateSlugs = []string{
	"locked-down",
	"web-safe-default",
	"dev-workstation",
	"cicd-runner",
	"incident-response-readonly",
}

// SeedAdminReviewSSPDTemplateSlugs — every preset except web-safe
// requires admin review (the default is routine; the others change
// posture materially).
var SeedAdminReviewSSPDTemplateSlugs = []string{
	"locked-down",
	"dev-workstation",
	"cicd-runner",
	"incident-response-readonly",
}

// SeedExpectedSSPDTemplateRowCount = 5.
const SeedExpectedSSPDTemplateRowCount = 5
