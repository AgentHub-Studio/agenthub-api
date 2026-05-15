package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityResponseTemplate is a single per-agent response structure
// template seeded by migration 000116. Response template rows encode the
// structured text patterns — greetings, error messages, and handoffs — that
// each capability agent uses to ensure consistent, predictable responses for
// common interaction patterns. Templates with {variable} placeholders require
// caller interpolation before rendering to the user.
type CoreCapabilityResponseTemplate struct {
	// ID is the auto-generated BIGSERIAL primary key.
	ID int64
	// AgentSlug identifies the capability agent that owns this template
	// (e.g. "core-researcher", "core-analyst", "core-planner").
	AgentSlug string
	// TemplateKey names the logical interaction pattern this template handles
	// (e.g. "greeting", "not_found", "ambiguity_prompt", "handoff").
	TemplateKey string
	// TemplateBody is the full template text. When HasPlaceholders is true the
	// body contains {variable} tokens that callers must interpolate before use.
	TemplateBody string
	// Description is a human-readable explanation of when this template is used
	// and what it communicates to the user.
	Description string
	// HasPlaceholders is true when TemplateBody contains {variable} placeholder
	// tokens that require interpolation before the text is rendered to the user.
	HasPlaceholders bool
	// CreatedAt is the UTC timestamp when the row was created.
	CreatedAt time.Time
}

// CoreCapabilityResponseTemplateLoader loads per-agent response templates from
// ah_core.capability_response_template. The table is seeded by migration 000116
// with 9 rows (3 per capability agent). Non-fatal when the table or schema is
// missing (supports fresh deployments before the migration runs).
type CoreCapabilityResponseTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityResponseTemplateLoader creates a
// CoreCapabilityResponseTemplateLoader backed by pool.
func NewCoreCapabilityResponseTemplateLoader(pool *pgxpool.Pool) *CoreCapabilityResponseTemplateLoader {
	return &CoreCapabilityResponseTemplateLoader{pool: pool}
}

// LoadCapabilityResponseTemplates returns all response template rows from
// ah_core.capability_response_template ordered by agent_slug, template_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityResponseTemplateLoader) LoadCapabilityResponseTemplates(ctx context.Context) ([]CoreCapabilityResponseTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, template_key, template_body, description, has_placeholders, created_at
		  FROM ah_core.capability_response_template
		 ORDER BY agent_slug, template_key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_response_template not accessible, response templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability response templates: %w", err)
	}
	defer rows.Close()

	var templates []CoreCapabilityResponseTemplate
	for rows.Next() {
		var t CoreCapabilityResponseTemplate
		if err := rows.Scan(
			&t.ID, &t.AgentSlug, &t.TemplateKey, &t.TemplateBody,
			&t.Description, &t.HasPlaceholders, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability response template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_response_template not accessible (post-iter), response templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability response templates: %w", err)
	}
	return templates, nil
}

// LoadResponseTemplatesForAgent returns the response template rows from
// ah_core.capability_response_template WHERE agent_slug = $1, ordered by
// template_key. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityResponseTemplateLoader) LoadResponseTemplatesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityResponseTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, template_key, template_body, description, has_placeholders, created_at
		  FROM ah_core.capability_response_template
		 WHERE agent_slug = $1
		 ORDER BY template_key`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_response_template not accessible, response templates unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query response templates for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var templates []CoreCapabilityResponseTemplate
	for rows.Next() {
		var t CoreCapabilityResponseTemplate
		if err := rows.Scan(
			&t.ID, &t.AgentSlug, &t.TemplateKey, &t.TemplateBody,
			&t.Description, &t.HasPlaceholders, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan response template for agent %q: %w", agentSlug, err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_response_template not accessible (post-iter), response templates unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate response templates for agent %q: %w", agentSlug, err)
	}
	return templates, nil
}

// GetResponseTemplate returns the response template for the given (agentSlug,
// templateKey) pair. Returns nil, nil when the row does not exist or the table
// is not accessible (non-fatal).
func (l *CoreCapabilityResponseTemplateLoader) GetResponseTemplate(ctx context.Context, agentSlug, templateKey string) (*CoreCapabilityResponseTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, template_key, template_body, description, has_placeholders, created_at
		  FROM ah_core.capability_response_template
		 WHERE agent_slug = $1
		   AND template_key = $2
		 LIMIT 1`

	var t CoreCapabilityResponseTemplate
	err = conn.QueryRow(ctx, query, agentSlug, templateKey).Scan(
		&t.ID, &t.AgentSlug, &t.TemplateKey, &t.TemplateBody,
		&t.Description, &t.HasPlaceholders, &t.CreatedAt,
	)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_response_template not accessible, returning nil",
				"agent_slug", agentSlug, "template_key", templateKey, "err", err)
			return nil, nil
		}
		// pgx returns "no rows in result set" when zero rows match.
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("core: get response template for agent %q key %q: %w", agentSlug, templateKey, err)
	}
	return &t, nil
}

// ============================================================
// Seed catalog constants — migration 000116 (2026-05-11).
// ============================================================

// SeedResponseTemplateCount is the expected total row count after migration
// 000116. Nine response template rows — three per capability agent (researcher,
// analyst, planner) — covering greeting, error/clarification, and handoff
// interaction patterns.
const SeedResponseTemplateCount = 9

// SeedResponseTemplateAgentCount is the number of capability agents that have
// seeded response template rows in migration 000116 (researcher, analyst,
// planner).
const SeedResponseTemplateAgentCount = 3

// SeedResponseTemplatesWithPlaceholdersCount is the number of seeded response
// template rows where has_placeholders=TRUE (ambiguity_prompt, confidence_footer,
// clarification_request, handoff). These templates contain {variable} tokens
// that callers must interpolate before rendering to the user.
const SeedResponseTemplatesWithPlaceholdersCount = 4

// Template key constants — migration 000116.

// SeedTemplateKeyGreeting is the template_key for the opening message shown to
// the user at the start of a session ("greeting"). All three capability agents
// have a greeting template (has_placeholders=FALSE).
const SeedTemplateKeyGreeting = "greeting"

// SeedTemplateKeyNotFound is the template_key for the response sent when search
// yields no useful results ("not_found"). Used by core-researcher only
// (has_placeholders=FALSE).
const SeedTemplateKeyNotFound = "not_found"

// SeedTemplateKeySourceDisclaimer is the template_key for the footer appended
// when core-researcher cites external sources ("source_disclaimer").
// has_placeholders=FALSE — the text is rendered verbatim.
const SeedTemplateKeySourceDisclaimer = "source_disclaimer"

// SeedTemplateKeyAmbiguity is the template_key for the clarification prompt
// sent by core-analyst when input is ambiguous before analysis begins
// ("ambiguity_prompt"). has_placeholders=TRUE — callers must fill
// {clarification_question} before rendering.
const SeedTemplateKeyAmbiguity = "ambiguity_prompt"

// SeedTemplateKeyConfidenceFooter is the template_key for the confidence
// metadata footer appended by core-analyst to analysis results
// ("confidence_footer"). has_placeholders=TRUE — callers must fill
// {confidence_level}, {data_points}, and {caveats} before rendering.
const SeedTemplateKeyConfidenceFooter = "confidence_footer"

// SeedTemplateKeyClarification is the template_key for the clarification
// request sent by core-planner before generating a plan
// ("clarification_request"). has_placeholders=TRUE — callers must fill
// {clarification_question} before rendering.
const SeedTemplateKeyClarification = "clarification_request"

// SeedTemplateKeyHandoff is the template_key for the closing message presented
// by core-planner after delivering a completed plan ("handoff").
// has_placeholders=TRUE — callers must fill {first_step} before rendering.
const SeedTemplateKeyHandoff = "handoff"
