package core

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SkillEffortLevelDefaultTemplate is one row from
// ah_core.skill_effort_level_template.
type SkillEffortLevelDefaultTemplate struct {
	ID             string
	Slug           string
	Label          string
	Description    string
	ThinkingTokens int
	RecommendedFor []string
	SortOrder      int
}

const SeedExpectedEffortLevelRowCount = 5

var SeedExpectedEffortLevelSlugs = []string{
	"lowest",
	"low",
	"medium",
	"high",
	"highest",
}

// SeedEffortLevelSlugRE is the kebab-case pattern every slug must satisfy.
var SeedEffortLevelSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SeedEffortLevelDefaultSlug is the slug used as the default for most skills.
const SeedEffortLevelDefaultSlug = "medium"

// SeedEffortLevelDisabledSlug is the slug where extended thinking is disabled.
const SeedEffortLevelDisabledSlug = "lowest"

// SeedEffortLevelMaxTokens is the thinking_tokens cap for the highest tier.
const SeedEffortLevelMaxTokens = 65536

// CoreSkillEffortLevelDefaultTemplateLoader loads effort-level templates from ah_core.
type CoreSkillEffortLevelDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSkillEffortLevelDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreSkillEffortLevelDefaultTemplateLoader(pool *pgxpool.Pool) *CoreSkillEffortLevelDefaultTemplateLoader {
	return &CoreSkillEffortLevelDefaultTemplateLoader{pool: pool}
}

const effortLevelLoadAllQuery = `
SELECT id, slug, label, description, thinking_tokens, recommended_for, sort_order
FROM ah_core.skill_effort_level_template
ORDER BY sort_order ASC, slug ASC
`

// LoadAll returns every template ordered by sort_order ASC, slug ASC.
// Returns an empty slice (not an error) when the table does not exist.
func (l *CoreSkillEffortLevelDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*SkillEffortLevelDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, effortLevelLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*SkillEffortLevelDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*SkillEffortLevelDefaultTemplate
	for rows.Next() {
		t := &SkillEffortLevelDefaultTemplate{}
		var recRaw []byte
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.ThinkingTokens, &recRaw, &t.SortOrder,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(recRaw, &t.RecommendedFor); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindBySlug returns the template matching slug, or (nil, false, nil) when not found.
func (l *CoreSkillEffortLevelDefaultTemplateLoader) FindBySlug(
	ctx context.Context, slug string,
) (*SkillEffortLevelDefaultTemplate, bool, error) {
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

// LoadByThinkingEnabled returns templates where extended thinking is active (tokens > 0).
func (l *CoreSkillEffortLevelDefaultTemplateLoader) LoadByThinkingEnabled(
	ctx context.Context,
) ([]*SkillEffortLevelDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*SkillEffortLevelDefaultTemplate
	for _, t := range all {
		if t.ThinkingTokens > 0 {
			out = append(out, t)
		}
	}
	return out, nil
}
