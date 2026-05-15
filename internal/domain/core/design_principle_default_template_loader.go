package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreDesignPrincipleTemplate is a platform-managed preset for one of the
// thirteen design principles from Table 1 (arXiv:2604.14228v1, §2.2).
type CoreDesignPrincipleTemplate struct {
	ID                 uuid.UUID
	Slug               string
	Label              string
	DesignQuestion     string
	ValuesServed       []string // human_authority|safety|reliability|capability|adaptability
	ReferencedSections []string // PDF section numbers
	SortOrder          int
}

// SeedExpectedDesignPrincipleSlugs is the canonical closed set from Table 1.
var SeedExpectedDesignPrincipleSlugs = []string{
	"deny_first_human_escalation",
	"graduated_trust_spectrum",
	"defense_in_depth_layered",
	"externalized_programmable_policy",
	"context_as_scarce_resource",
	"append_only_durable_state",
	"minimal_scaffolding_maximal_harness",
	"values_over_rules",
	"composable_multi_mechanism",
	"reversibility_weighted_risk",
	"transparent_file_based",
	"isolated_subagent_boundaries",
	"graceful_recovery_resilience",
}

// SeedExpectedDesignPrincipleRowCount matches the migration INSERT count.
const SeedExpectedDesignPrincipleRowCount = 13

// SeedDesignPrincipleSafetyAuthorityServingSlugs are principles that serve both safety and authority.
var SeedDesignPrincipleSafetyAuthorityServingSlugs = []string{
	"deny_first_human_escalation",
	"defense_in_depth_layered",
	"externalized_programmable_policy",
}

// SeedDesignPrincipleCapabilityServingSlugs are principles that serve capability.
var SeedDesignPrincipleCapabilityServingSlugs = []string{
	"context_as_scarce_resource",
	"minimal_scaffolding_maximal_harness",
	"values_over_rules",
	"composable_multi_mechanism",
	"reversibility_weighted_risk",
	"isolated_subagent_boundaries",
	"graceful_recovery_resilience",
}

// CoreDesignPrincipleDefaultTemplateLoader loads Table 1 principle presets from ah_core.
type CoreDesignPrincipleDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreDesignPrincipleDefaultTemplateLoader creates a loader.
func NewCoreDesignPrincipleDefaultTemplateLoader(pool *pgxpool.Pool) *CoreDesignPrincipleDefaultTemplateLoader {
	return &CoreDesignPrincipleDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all thirteen principles ordered by sort_order (Table 1 row order).
func (l *CoreDesignPrincipleDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreDesignPrincipleTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, design_question,
		       values_served, referenced_sections, sort_order
		  FROM ah_core.design_principle_template
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.design_principle_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query design_principle_template: %w", err)
	}
	defer rows.Close()

	var principles []CoreDesignPrincipleTemplate
	for rows.Next() {
		var t CoreDesignPrincipleTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.DesignQuestion,
			&t.ValuesServed, &t.ReferencedSections, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan design_principle_template: %w", err)
		}
		principles = append(principles, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate design_principle_template: %w", err)
	}
	return principles, nil
}

// FindBySlug returns one principle template by slug.
func (l *CoreDesignPrincipleDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreDesignPrincipleTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreDesignPrincipleTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreDesignPrincipleTemplate{}, false, nil
}

// LoadServingValue returns principles that include the given design value in values_served.
func (l *CoreDesignPrincipleDefaultTemplateLoader) LoadServingValue(ctx context.Context, value string) ([]CoreDesignPrincipleTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreDesignPrincipleTemplate
	for _, t := range all {
		for _, v := range t.ValuesServed {
			if v == value {
				matched = append(matched, t)
				break
			}
		}
	}
	return matched, nil
}
