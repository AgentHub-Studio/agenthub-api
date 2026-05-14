package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreOutputStyle represents a platform-managed default output style from
// ah_core.output_style. Output styles are response-formatting templates
// the runner prepends to the system prompt: structure, length, register,
// formatting.
//
// Inspired by PDF arXiv:2604.14228v1 Section 6.1 + 7.
type CoreOutputStyle struct {
	ID             uuid.UUID
	Name           string
	Slug           string
	Description    string
	PromptTemplate string
	OutputFormat   string // markdown | plain | json
	MaxWords       int    // 0 = unbounded
	Audience       string // general | technical | executive | beginner
	IsDefault      bool
	IsActive       bool
	SortOrder      int
}

// CoreOutputStyleLoader loads platform-managed default output styles.
// Like other core loaders, non-fatal when schema missing.
type CoreOutputStyleLoader struct {
	pool *pgxpool.Pool
}

// NewCoreOutputStyleLoader creates a CoreOutputStyleLoader backed by the
// given pool.
func NewCoreOutputStyleLoader(pool *pgxpool.Pool) *CoreOutputStyleLoader {
	return &CoreOutputStyleLoader{pool: pool}
}

// LoadAll returns all active output styles, ordered by sort_order then slug.
func (l *CoreOutputStyleLoader) LoadAll(ctx context.Context) ([]CoreOutputStyle, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       prompt_template, output_format, max_words,
		       audience, is_default, is_active, sort_order
		  FROM ah_core.output_style
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.output_style not accessible, core output styles unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query output styles: %w", err)
	}
	defer rows.Close()

	var styles []CoreOutputStyle
	for rows.Next() {
		var s CoreOutputStyle
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Slug, &s.Description,
			&s.PromptTemplate, &s.OutputFormat, &s.MaxWords,
			&s.Audience, &s.IsDefault, &s.IsActive, &s.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan output style: %w", err)
		}
		styles = append(styles, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.output_style not accessible (post-iter)", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate output styles: %w", err)
	}
	return styles, nil
}

// FindBySlug returns one style by slug.
func (l *CoreOutputStyleLoader) FindBySlug(ctx context.Context, slug string) (CoreOutputStyle, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreOutputStyle{}, false, err
	}
	for _, s := range all {
		if s.Slug == slug {
			return s, true, nil
		}
	}
	return CoreOutputStyle{}, false, nil
}

// FindDefault returns the platform default style (is_default=true).
// At most one row has is_default=true (enforced by unique partial index).
// Returns (zero, false, nil) if no default is configured.
func (l *CoreOutputStyleLoader) FindDefault(ctx context.Context) (CoreOutputStyle, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreOutputStyle{}, false, err
	}
	for _, s := range all {
		if s.IsDefault {
			return s, true, nil
		}
	}
	return CoreOutputStyle{}, false, nil
}

// SeedExpectedOutputStyleSlugs is the canonical list of slugs the seed
// migration 000011_seed_output_styles installs. Tests assert this list
// matches actual rows.
var SeedExpectedOutputStyleSlugs = []string{
	"conversational", // DEFAULT
	"concise",
	"structured",
	"technical",
	"verbose",
	"tutorial",
	"executive",
	"json_only",
}

// SeedExpectedOutputStyleFormats is the closed set of output_format values
// the seed uses.
var SeedExpectedOutputStyleFormats = []string{
	"markdown",
	"json",
}

// SeedExpectedOutputStyleAudiences is the closed set of audience values
// the seed uses. Surfaces in the UI picker as filter facets.
var SeedExpectedOutputStyleAudiences = []string{
	"general",
	"technical",
	"executive",
	"beginner",
}

// SeedDefaultOutputStyleSlug is the slug of the platform default style.
// Auto-selected for new agents/sessions when no tenant override exists.
const SeedDefaultOutputStyleSlug = "conversational"
