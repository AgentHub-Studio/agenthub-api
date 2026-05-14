package core

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreMemoryIncludeDirectiveDefaultTemplate is a starter @include{key}
// directive paired with CTX-007 MemoryIncludeResolver. Tenants can
// reference these keys from their memory facts.
type CoreMemoryIncludeDirectiveDefaultTemplate struct {
	ID                   uuid.UUID
	Slug                 string
	IncludeKey           string // matches CTX-007 regex [a-zA-Z0-9._-]+
	ContentTemplate      string
	Category             string
	Description          string
	ContainsPlaceholders bool
	IsRecommended        bool
	IsActive             bool
	SortOrder            int
}

// MemoryIncludeKeyRE replicates the CTX-007 includePattern's key
// constraint byte-for-byte. Seed authors and admin tooling consult
// this to validate keys before insertion.
var MemoryIncludeKeyRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// CoreMemoryIncludeDirectiveDefaultTemplateLoader loads templates.
type CoreMemoryIncludeDirectiveDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreMemoryIncludeDirectiveDefaultTemplateLoader creates the loader.
func NewCoreMemoryIncludeDirectiveDefaultTemplateLoader(pool *pgxpool.Pool) *CoreMemoryIncludeDirectiveDefaultTemplateLoader {
	return &CoreMemoryIncludeDirectiveDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreMemoryIncludeDirectiveDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreMemoryIncludeDirectiveDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, include_key, content_template, category,
		       description, contains_placeholders, is_recommended,
		       is_active, sort_order
		  FROM ah_core.memory_include_directive_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.memory_include_directive_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query memory_include_directive_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreMemoryIncludeDirectiveDefaultTemplate
	for rows.Next() {
		var t CoreMemoryIncludeDirectiveDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.IncludeKey, &t.ContentTemplate, &t.Category,
			&t.Description, &t.ContainsPlaceholders, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan memory_include_directive_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate memory_include_directive_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template by slug.
func (l *CoreMemoryIncludeDirectiveDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreMemoryIncludeDirectiveDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreMemoryIncludeDirectiveDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreMemoryIncludeDirectiveDefaultTemplate{}, false, nil
}

// FindByIncludeKey returns one template by include_key.
func (l *CoreMemoryIncludeDirectiveDefaultTemplateLoader) FindByIncludeKey(ctx context.Context, key string) (CoreMemoryIncludeDirectiveDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreMemoryIncludeDirectiveDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.IncludeKey == key {
			return t, true, nil
		}
	}
	return CoreMemoryIncludeDirectiveDefaultTemplate{}, false, nil
}

// LoadByCategory filters by category.
func (l *CoreMemoryIncludeDirectiveDefaultTemplateLoader) LoadByCategory(ctx context.Context, category string) ([]CoreMemoryIncludeDirectiveDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMemoryIncludeDirectiveDefaultTemplate
	for _, t := range all {
		if t.Category == category {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LookupMap builds a map suitable for handing directly to the CTX-007
// MemoryIncludeResolver SetLookup method.
func (l *CoreMemoryIncludeDirectiveDefaultTemplateLoader) LookupMap(ctx context.Context) (map[string]string, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(all))
	for _, t := range all {
		out[t.IncludeKey] = t.ContentTemplate
	}
	return out, nil
}

// SeedExpectedMIDTemplateSlugs is the closed canonical set.
var SeedExpectedMIDTemplateSlugs = []string{
	"core-org-identity",
	"core-cite-format",
	"core-code-style-preamble",
	"core-safety-do-not",
	"core-locale-pt-br-summary",
	"core-runner-handoff",
}

// SeedExpectedMIDTemplateIncludeKeys — all keys conform to CTX-007 regex.
var SeedExpectedMIDTemplateIncludeKeys = []string{
	"core.org.identity",
	"core.cite.format",
	"core.code.style.preamble",
	"core.safety.do_not",
	"core.locale.pt_br.summary",
	"core.runner.handoff",
}

// SeedExpectedMIDTemplateCategories — closed category set.
var SeedExpectedMIDTemplateCategories = []string{
	"organization", "citation", "code_style",
	"safety", "locale", "handoff",
}

// SeedPlaceholderMIDTemplateSlugs — templates whose content uses
// {{placeholder}} tokens (resolved before include expansion).
var SeedPlaceholderMIDTemplateSlugs = []string{
	"core-org-identity",
}

// SeedRecommendedMIDTemplateSlugs — all 6 directives recommended.
var SeedRecommendedMIDTemplateSlugs = SeedExpectedMIDTemplateSlugs

// SeedExpectedMIDTemplateRowCount = 6.
const SeedExpectedMIDTemplateRowCount = 6
