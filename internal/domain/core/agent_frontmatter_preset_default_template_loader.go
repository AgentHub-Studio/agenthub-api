package core

import (
	"context"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentFrontmatterPresetDefaultTemplate is one row from
// ah_core.agent_frontmatter_preset_template.
type AgentFrontmatterPresetDefaultTemplate struct {
	ID              string
	Slug            string
	Label           string
	Description     string
	FrontmatterYAML string
	SortOrder       int
}

// Seed-time canonical constants.

const SeedExpectedAFPTRowCount = 6

var SeedExpectedAFPTSlugs = []string{
	"fast-responder",
	"careful-analyst",
	"creative-writer",
	"security-auditor",
	"data-extractor",
	"research-assistant",
}

// SeedAFPTSlugRE is the kebab-case pattern every slug must satisfy.
var SeedAFPTSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SeedHighPrioritySlugs — presets with priority > 0 in their frontmatter.
var SeedHighPrioritySlugs = []string{
	"security-auditor",
}

// CoreAgentFrontmatterPresetDefaultTemplateLoader loads templates from ah_core.
type CoreAgentFrontmatterPresetDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreAgentFrontmatterPresetDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool *pgxpool.Pool) *CoreAgentFrontmatterPresetDefaultTemplateLoader {
	return &CoreAgentFrontmatterPresetDefaultTemplateLoader{pool: pool}
}

const afptLoadAllQuery = `
SELECT id, slug, label, description, frontmatter_yaml, sort_order
FROM ah_core.agent_frontmatter_preset_template
ORDER BY sort_order ASC, slug ASC
`

// LoadAll returns every template ordered by sort_order ASC, slug ASC.
// Returns an empty slice (not an error) when the table does not exist.
func (l *CoreAgentFrontmatterPresetDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*AgentFrontmatterPresetDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, afptLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*AgentFrontmatterPresetDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*AgentFrontmatterPresetDefaultTemplate
	for rows.Next() {
		t := &AgentFrontmatterPresetDefaultTemplate{}
		if err := rows.Scan(&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.FrontmatterYAML, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindBySlug returns the template matching slug, or (nil, false, nil) when not found.
func (l *CoreAgentFrontmatterPresetDefaultTemplateLoader) FindBySlug(
	ctx context.Context, slug string,
) (*AgentFrontmatterPresetDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return nil, false, nil
}
