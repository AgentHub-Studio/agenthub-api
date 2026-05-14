package core

import (
	"context"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LearningAnnotationDefaultTemplate is one row from
// ah_core.learning_annotation_template.
type LearningAnnotationDefaultTemplate struct {
	ID                  string
	Slug                string
	Kind                string
	Audience            string
	Title               string
	Content             string
	RelatedFeatureSlugs []string
	SortOrder           int
}

// Seed-time canonical constants — loaders and integration tests reference
// the same source of truth rather than hard-coding literals in both places.

const SeedExpectedLATTemplateRowCount = 6

var SeedExpectedLATTemplateSlugs = []string{
	"agent-tool-binding-pattern",
	"no-system-prompt-antipattern",
	"skill-reuse-tip",
	"context-budget-optimization",
	"knowledge-base-chunking-gap",
	"escalation-workflow-pattern",
}

// SeedBeginnerLATTemplateSlugs — audience=beginner.
var SeedBeginnerLATTemplateSlugs = []string{
	"agent-tool-binding-pattern",
	"no-system-prompt-antipattern",
}

// SeedIntermediateLATTemplateSlugs — audience=intermediate.
var SeedIntermediateLATTemplateSlugs = []string{
	"skill-reuse-tip",
	"context-budget-optimization",
	"knowledge-base-chunking-gap",
}

// SeedAdvancedLATTemplateSlugs — audience=advanced.
var SeedAdvancedLATTemplateSlugs = []string{
	"escalation-workflow-pattern",
}

// SeedExpectedLATTemplateKinds — closed set of kinds present in the seed.
var SeedExpectedLATTemplateKinds = []string{
	"pattern",
	"anti_pattern",
	"tip",
	"optimization",
	"knowledge_gap",
}

// SeedExpectedLATTemplateAudiences — closed set of audiences present in the seed.
var SeedExpectedLATTemplateAudiences = []string{
	"beginner",
	"intermediate",
	"advanced",
}

// LearningAnnotationTemplateSeedSlugRE is the kebab-case pattern every slug must satisfy.
var LearningAnnotationTemplateSeedSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// CoreLearningAnnotationDefaultTemplateLoader loads templates from ah_core.
type CoreLearningAnnotationDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreLearningAnnotationDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreLearningAnnotationDefaultTemplateLoader(pool *pgxpool.Pool) *CoreLearningAnnotationDefaultTemplateLoader {
	return &CoreLearningAnnotationDefaultTemplateLoader{pool: pool}
}

const latLoadAllQuery = `
SELECT id, slug, kind, audience, title, content,
       COALESCE(
           (SELECT array_agg(v) FROM jsonb_array_elements_text(related_feature_slugs) v),
           '{}'::TEXT[]
       ),
       sort_order
FROM ah_core.learning_annotation_template
ORDER BY sort_order ASC, slug ASC
`

// LoadAll returns every template ordered by sort_order ASC, slug ASC.
// Returns an empty slice — not an error — when the table does not exist.
func (l *CoreLearningAnnotationDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*LearningAnnotationDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, latLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*LearningAnnotationDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*LearningAnnotationDefaultTemplate
	for rows.Next() {
		t := &LearningAnnotationDefaultTemplate{}
		if err := rows.Scan(&t.ID, &t.Slug, &t.Kind, &t.Audience,
			&t.Title, &t.Content, &t.RelatedFeatureSlugs, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindBySlug returns the template matching slug, or (nil, false, nil) when not found.
func (l *CoreLearningAnnotationDefaultTemplateLoader) FindBySlug(
	ctx context.Context, slug string,
) (*LearningAnnotationDefaultTemplate, bool, error) {
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

// LoadByKind returns all templates whose kind equals the given value,
// in sort_order ASC, slug ASC order.
func (l *CoreLearningAnnotationDefaultTemplateLoader) LoadByKind(
	ctx context.Context, kind string,
) ([]*LearningAnnotationDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*LearningAnnotationDefaultTemplate
	for _, t := range all {
		if t.Kind == kind {
			out = append(out, t)
		}
	}
	return out, nil
}

// LoadByAudience returns all templates matching the exact audience value,
// in sort_order ASC, slug ASC order.
func (l *CoreLearningAnnotationDefaultTemplateLoader) LoadByAudience(
	ctx context.Context, audience string,
) ([]*LearningAnnotationDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*LearningAnnotationDefaultTemplate
	for _, t := range all {
		if t.Audience == audience {
			out = append(out, t)
		}
	}
	return out, nil
}
