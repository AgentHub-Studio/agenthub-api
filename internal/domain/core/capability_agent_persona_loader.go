package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentPersonaLoader loads per-capability agent persona profiles
// from ah_core.capability_agent_persona. These 3 rows (seeded by migration
// 000104) provide personality/identity profiles for each capability agent,
// adapted from the Claude Code Identity Model §3:
//
//   - capability-researcher-persona  (core-researcher, tone: curious)
//   - capability-analyst-persona     (core-analyst,    tone: precise)
//   - capability-planner-persona     (core-planner,    tone: pragmatic)
//
// Non-fatal when the ah_core schema or the capability_agent_persona table
// is missing — supports fresh deployments where migration 000104 has not yet run.
type CoreCapabilityAgentPersonaLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentPersonaLoader creates a
// CoreCapabilityAgentPersonaLoader backed by pool.
func NewCoreCapabilityAgentPersonaLoader(pool *pgxpool.Pool) *CoreCapabilityAgentPersonaLoader {
	return &CoreCapabilityAgentPersonaLoader{pool: pool}
}

// CoreAgentPersona is a single capability agent identity profile.
// Captures the slug, the agent it belongs to, the dominant tone, a set of
// personality trait labels, communication style description, primary expertise
// domain, whether it is a recommended default, and its display sort position.
type CoreAgentPersona struct {
	Slug               string
	AgentSlug          string
	Tone               string
	Traits             []string
	CommunicationStyle string
	ExpertiseDomain    string
	IsRecommended      bool
	SortOrder          int
}

// LoadCapabilityAgentPersonas returns all capability agent persona rows from
// ah_core.capability_agent_persona WHERE slug = ANY($1), ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentPersonaLoader) LoadCapabilityAgentPersonas(ctx context.Context) ([]CoreAgentPersona, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, tone, traits, communication_style, expertise_domain, is_recommended, sort_order
		  FROM ah_core.capability_agent_persona
		 WHERE slug = ANY($1)
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityAgentPersonaSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_persona not accessible, capability agent personas unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent personas: %w", err)
	}
	defer rows.Close()

	var personas []CoreAgentPersona
	for rows.Next() {
		var p CoreAgentPersona
		var traits []string
		if err := rows.Scan(
			&p.Slug, &p.AgentSlug, &p.Tone, &traits,
			&p.CommunicationStyle, &p.ExpertiseDomain, &p.IsRecommended, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent persona: %w", err)
		}
		p.Traits = traits
		personas = append(personas, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_persona not accessible (post-iter), capability agent personas unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent personas: %w", err)
	}
	return personas, nil
}

// FindPersonaByAgentSlug returns the first capability agent persona row where
// agent_slug matches the given slug. Returns nil, nil when the row is not found
// or the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentPersonaLoader) FindPersonaByAgentSlug(ctx context.Context, agentSlug string) (*CoreAgentPersona, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, tone, traits, communication_style, expertise_domain, is_recommended, sort_order
		  FROM ah_core.capability_agent_persona
		 WHERE agent_slug = $1
		 LIMIT 1`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_persona not accessible, find persona unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query persona by agent slug: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			if isUndefinedRelation(err) {
				slog.WarnContext(ctx, "core: ah_core.capability_agent_persona not accessible (post-iter), find persona unavailable", "err", err, "agent_slug", agentSlug)
				return nil, nil
			}
			return nil, fmt.Errorf("core: iterate persona by agent slug: %w", err)
		}
		return nil, nil
	}

	var p CoreAgentPersona
	var traits []string
	if err := rows.Scan(
		&p.Slug, &p.AgentSlug, &p.Tone, &traits,
		&p.CommunicationStyle, &p.ExpertiseDomain, &p.IsRecommended, &p.SortOrder,
	); err != nil {
		return nil, fmt.Errorf("core: scan persona by agent slug: %w", err)
	}
	p.Traits = traits
	return &p, nil
}

// ============================================================
// Seed catalog constants — migration 000104 (2026-05-11).
// ============================================================

// SeedCapabilityAgentPersonaCount is the expected total row count after
// migration 000104. Three persona rows, one per capability agent
// (researcher, analyst, planner).
const SeedCapabilityAgentPersonaCount = 3

// SeedCapabilityAgentPersonaSlugs is the canonical closed set of capability
// agent persona slugs seeded by migration 000104.
var SeedCapabilityAgentPersonaSlugs = []string{
	"capability-researcher-persona",
	"capability-analyst-persona",
	"capability-planner-persona",
}

// Individual slug constants for each seeded capability agent persona.

// SeedResearcherPersonaSlug is the slug for the researcher capability agent
// persona (tone: curious, agent: core-researcher).
const SeedResearcherPersonaSlug = "capability-researcher-persona"

// SeedAnalystPersonaSlug is the slug for the analyst capability agent
// persona (tone: precise, agent: core-analyst).
const SeedAnalystPersonaSlug = "capability-analyst-persona"

// SeedPlannerPersonaSlug is the slug for the planner capability agent
// persona (tone: pragmatic, agent: core-planner).
const SeedPlannerPersonaSlug = "capability-planner-persona"

// Tone constants for the three seeded capability agent personas.

// SeedResearcherTone is the dominant tone for the researcher persona —
// curious inquiry to drive thorough information gathering.
const SeedResearcherTone = "curious"

// SeedAnalystTone is the dominant tone for the analyst persona —
// precise focus for evidence-based analysis and pattern extraction.
const SeedAnalystTone = "precise"

// SeedPlannerTone is the dominant tone for the planner persona —
// pragmatic orientation for actionable task decomposition.
const SeedPlannerTone = "pragmatic"

// Trait count constants for the seeded personas.

// SeedResearcherTraitCount is the number of traits for the researcher persona (4).
const SeedResearcherTraitCount = 4

// SeedAnalystTraitCount is the number of traits for the analyst persona (4).
const SeedAnalystTraitCount = 4

// SeedPlannerTraitCount is the number of traits for the planner persona (4).
const SeedPlannerTraitCount = 4

// SeedTotalTraitCount is the total trait count across all 3 seeded personas (12).
const SeedTotalTraitCount = 12
