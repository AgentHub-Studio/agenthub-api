package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreSubagentReturnSummaryDefaultTemplate is a platform-managed
// blueprint paired with SUB-010 SubagentReturnSummary. Each template
// captures a proven outcome × use_case shape plus redaction posture.
type CoreSubagentReturnSummaryDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetOutcome            string
	TargetUseCase            string
	SampleFindingsJSON       string
	SampleArtifactsJSON      string
	SampleNextStepsJSON      string
	RedactFindings           bool
	RedactArtifacts          bool
	RedactTranscriptHash     bool
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// SampleFindings parses the JSONB array.
func (t CoreSubagentReturnSummaryDefaultTemplate) SampleFindings() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SampleFindingsJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_return_summary_template %s: invalid sample_findings: %w", t.Slug, err)
	}
	return out, nil
}

// SampleArtifacts parses the JSONB array.
func (t CoreSubagentReturnSummaryDefaultTemplate) SampleArtifacts() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SampleArtifactsJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_return_summary_template %s: invalid sample_artifacts: %w", t.Slug, err)
	}
	return out, nil
}

// SampleNextSteps parses the JSONB array.
func (t CoreSubagentReturnSummaryDefaultTemplate) SampleNextSteps() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.SampleNextStepsJSON), &out); err != nil {
		return nil, fmt.Errorf("subagent_return_summary_template %s: invalid sample_next_steps: %w", t.Slug, err)
	}
	return out, nil
}

// CoreSubagentReturnSummaryDefaultTemplateLoader loads templates.
type CoreSubagentReturnSummaryDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSubagentReturnSummaryDefaultTemplateLoader creates the loader.
func NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool *pgxpool.Pool) *CoreSubagentReturnSummaryDefaultTemplateLoader {
	return &CoreSubagentReturnSummaryDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreSubagentReturnSummaryDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreSubagentReturnSummaryDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_outcome, target_use_case,
		       sample_findings::text, sample_artifacts::text, sample_next_steps::text,
		       redact_findings, redact_artifacts, redact_transcript_hash,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.subagent_return_summary_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.subagent_return_summary_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query subagent_return_summary_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreSubagentReturnSummaryDefaultTemplate
	for rows.Next() {
		var t CoreSubagentReturnSummaryDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetOutcome, &t.TargetUseCase,
			&t.SampleFindingsJSON, &t.SampleArtifactsJSON, &t.SampleNextStepsJSON,
			&t.RedactFindings, &t.RedactArtifacts, &t.RedactTranscriptHash,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan subagent_return_summary_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate subagent_return_summary_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreSubagentReturnSummaryDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreSubagentReturnSummaryDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreSubagentReturnSummaryDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreSubagentReturnSummaryDefaultTemplate{}, false, nil
}

// LoadByOutcome filters by SUB-010 SubagentReturnOutcome.
func (l *CoreSubagentReturnSummaryDefaultTemplateLoader) LoadByOutcome(ctx context.Context, outcome string) ([]CoreSubagentReturnSummaryDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentReturnSummaryDefaultTemplate
	for _, t := range all {
		if t.TargetOutcome == outcome {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case.
func (l *CoreSubagentReturnSummaryDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreSubagentReturnSummaryDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentReturnSummaryDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreSubagentReturnSummaryDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreSubagentReturnSummaryDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSubagentReturnSummaryDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedSRSDTemplateSlugs is the closed canonical set.
var SeedExpectedSRSDTemplateSlugs = []string{
	"success-with-artifacts",
	"partial-needs-followup",
	"failed-error",
	"aborted-by-parent",
	"audit-with-redaction",
}

// SeedExpectedSRSDTemplateOutcomes matches SUB-010 SubagentReturnOutcome
// enum byte-for-byte (4 outcomes; success appears twice — routine and
// audit-redacted).
var SeedExpectedSRSDTemplateOutcomes = []string{
	"success", "partial", "failed", "aborted",
}

// SeedExpectedSRSDTemplateUseCases is the closed use-case set.
var SeedExpectedSRSDTemplateUseCases = []string{
	"task_completion", "incremental_progress",
	"error_diagnosis", "interruption_handling", "compliance_export",
}

// SeedExpectedSRSDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedSRSDTemplateTenantKinds = []string{"general"}

// SeedRecommendedSRSDTemplateSlugs — all 5 are recommended.
var SeedRecommendedSRSDTemplateSlugs = []string{
	"success-with-artifacts",
	"partial-needs-followup",
	"failed-error",
	"aborted-by-parent",
	"audit-with-redaction",
}

// SeedAdminReviewSRSDTemplateSlugs — failed/aborted/audit require
// admin review (success-with-artifacts + partial are routine).
var SeedAdminReviewSRSDTemplateSlugs = []string{
	"failed-error",
	"aborted-by-parent",
	"audit-with-redaction",
}

// SeedExpectedSRSDTemplateRowCount = 5.
const SeedExpectedSRSDTemplateRowCount = 5
