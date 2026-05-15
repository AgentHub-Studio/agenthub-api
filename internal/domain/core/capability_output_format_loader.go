package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityOutputFormat is a single per-agent output format preference
// entry seeded by migration 000115. Output format rows encode how each
// capability agent structures and styles its responses — covering default
// format, citation style, number formatting, uncertainty notation, and other
// presentation preferences adapted from agent output style templates seeded in
// migration 000095.
type CoreCapabilityOutputFormat struct {
	// ID is the auto-generated BIGSERIAL primary key.
	ID int64
	// AgentSlug identifies the capability agent that owns this format setting
	// (e.g. "core-researcher", "core-analyst", "core-planner").
	AgentSlug string
	// FormatKey names the format dimension
	// (e.g. "default_format", "citation_style", "number_format").
	FormatKey string
	// FormatValue is the chosen value for the format dimension
	// (e.g. "markdown", "inline", "grouped").
	FormatValue string
	// Description is a human-readable explanation of what this format setting
	// means and why it was chosen for this agent.
	Description string
	// DisplayOrder controls presentation order within an agent's format
	// settings. Seed values: 1, 2, 3 per agent (sequential).
	DisplayOrder int
	// CreatedAt is the UTC timestamp when the row was created.
	CreatedAt time.Time
}

// CoreCapabilityOutputFormatLoader loads per-agent output format preferences
// from ah_core.capability_output_format. The table is seeded by migration
// 000115 with 9 rows (3 per capability agent). Non-fatal when the table or
// schema is missing (supports fresh deployments before the migration runs).
type CoreCapabilityOutputFormatLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityOutputFormatLoader creates a
// CoreCapabilityOutputFormatLoader backed by pool.
func NewCoreCapabilityOutputFormatLoader(pool *pgxpool.Pool) *CoreCapabilityOutputFormatLoader {
	return &CoreCapabilityOutputFormatLoader{pool: pool}
}

// LoadCapabilityOutputFormats returns all output format rows from
// ah_core.capability_output_format ordered by agent_slug, display_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityOutputFormatLoader) LoadCapabilityOutputFormats(ctx context.Context) ([]CoreCapabilityOutputFormat, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, format_key, format_value, description, display_order, created_at
		  FROM ah_core.capability_output_format
		 ORDER BY agent_slug, display_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_output_format not accessible, output formats unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability output formats: %w", err)
	}
	defer rows.Close()

	var formats []CoreCapabilityOutputFormat
	for rows.Next() {
		var f CoreCapabilityOutputFormat
		if err := rows.Scan(
			&f.ID, &f.AgentSlug, &f.FormatKey, &f.FormatValue,
			&f.Description, &f.DisplayOrder, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability output format: %w", err)
		}
		formats = append(formats, f)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_output_format not accessible (post-iter), output formats unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability output formats: %w", err)
	}
	return formats, nil
}

// LoadOutputFormatsForAgent returns the output format rows from
// ah_core.capability_output_format WHERE agent_slug = $1, ordered by
// display_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityOutputFormatLoader) LoadOutputFormatsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityOutputFormat, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, format_key, format_value, description, display_order, created_at
		  FROM ah_core.capability_output_format
		 WHERE agent_slug = $1
		 ORDER BY display_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_output_format not accessible, output formats unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query output formats for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var formats []CoreCapabilityOutputFormat
	for rows.Next() {
		var f CoreCapabilityOutputFormat
		if err := rows.Scan(
			&f.ID, &f.AgentSlug, &f.FormatKey, &f.FormatValue,
			&f.Description, &f.DisplayOrder, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan output format for agent %q: %w", agentSlug, err)
		}
		formats = append(formats, f)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_output_format not accessible (post-iter), output formats unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate output formats for agent %q: %w", agentSlug, err)
	}
	return formats, nil
}

// GetOutputFormatValue returns the format_value for the given (agentSlug,
// formatKey) pair. The second return value is true when the row exists, false
// when it is absent (not found). Returns "", false, nil when the table is not
// accessible (non-fatal).
func (l *CoreCapabilityOutputFormatLoader) GetOutputFormatValue(ctx context.Context, agentSlug, formatKey string) (string, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return "", false, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT format_value
		  FROM ah_core.capability_output_format
		 WHERE agent_slug = $1
		   AND format_key = $2
		 LIMIT 1`

	var value string
	err = conn.QueryRow(ctx, query, agentSlug, formatKey).Scan(&value)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_output_format not accessible, returning not-found",
				"agent_slug", agentSlug, "format_key", formatKey, "err", err)
			return "", false, nil
		}
		// pgx returns pgx.ErrNoRows when the query returns zero rows.
		if err.Error() == "no rows in result set" {
			return "", false, nil
		}
		return "", false, fmt.Errorf("core: get output format value for agent %q key %q: %w", agentSlug, formatKey, err)
	}
	return value, true, nil
}

// ============================================================
// Seed catalog constants — migration 000115 (2026-05-11).
// ============================================================

// SeedOutputFormatCount is the expected total row count after migration 000115.
// Nine output format rows — three per capability agent (researcher, analyst,
// planner), each encoding a key/value pair that governs how the agent
// structures and styles its responses.
const SeedOutputFormatCount = 9

// SeedOutputFormatAgentCount is the number of capability agents that have
// seeded output format rows in migration 000115 (researcher, analyst, planner).
const SeedOutputFormatAgentCount = 3

// SeedResearcherOutputFormatCount is the number of output format rows seeded
// for core-researcher (3: default_format + citation_style + summary_position).
const SeedResearcherOutputFormatCount = 3

// SeedAnalystOutputFormatCount is the number of output format rows seeded for
// core-analyst (3: default_format + number_format + uncertainty_notation).
const SeedAnalystOutputFormatCount = 3

// SeedPlannerOutputFormatCount is the number of output format rows seeded for
// core-planner (3: default_format + step_numbering + code_blocks).
const SeedPlannerOutputFormatCount = 3

// Format key constants — migration 000115.

// SeedFormatKeyDefault is the format_key for the primary response format
// dimension (e.g. "markdown", "structured", "checklist").
const SeedFormatKeyDefault = "default_format"

// SeedFormatKeyCitation is the format_key for the citation style dimension
// used by core-researcher ("inline" → [Source: URL] notation).
const SeedFormatKeyCitation = "citation_style"

// SeedFormatKeySummaryPosition is the format_key for the summary position
// dimension used by core-researcher ("top" → executive summary at top).
const SeedFormatKeySummaryPosition = "summary_position"

// SeedFormatKeyNumberFormat is the format_key for the number formatting
// dimension used by core-analyst ("grouped" → thousands separator).
const SeedFormatKeyNumberFormat = "number_format"

// SeedFormatKeyUncertainty is the format_key for the uncertainty notation
// dimension used by core-analyst ("range" → e.g. 80-90%).
const SeedFormatKeyUncertainty = "uncertainty_notation"

// SeedFormatKeyStepNumbering is the format_key for the step numbering
// dimension used by core-planner ("sequential" → 1. 2. 3.).
const SeedFormatKeyStepNumbering = "step_numbering"

// SeedFormatKeyCodeBlocks is the format_key for the code block formatting
// dimension used by core-planner ("always" → fenced blocks with language tag).
const SeedFormatKeyCodeBlocks = "code_blocks"

// Per-agent default format value constants — migration 000115.

// SeedResearcherDefaultFormat is the format_value of the default_format row
// seeded for core-researcher. Markdown enables rich formatting with headers,
// bullet lists, and code spans that improve readability for research outputs.
const SeedResearcherDefaultFormat = "markdown"

// SeedAnalystDefaultFormat is the format_value of the default_format row
// seeded for core-analyst. Structured format uses labeled headings for each
// response section, making analysis outputs easy to navigate and verify.
const SeedAnalystDefaultFormat = "structured"

// SeedPlannerDefaultFormat is the format_value of the default_format row
// seeded for core-planner. Checklist format presents tasks as markdown [ ]
// boxes, making plans immediately actionable without additional formatting.
const SeedPlannerDefaultFormat = "checklist"

// Display order constants — migration 000115.

// SeedOutputFormatDisplayOrderFirst is the display_order value (1) for the
// primary format setting of each agent (always the default_format key).
const SeedOutputFormatDisplayOrderFirst = 1

// SeedOutputFormatDisplayOrderSecond is the display_order value (2) for the
// secondary format setting of each agent.
const SeedOutputFormatDisplayOrderSecond = 2

// SeedOutputFormatDisplayOrderThird is the display_order value (3) for the
// tertiary format setting of each agent.
const SeedOutputFormatDisplayOrderThird = 3
