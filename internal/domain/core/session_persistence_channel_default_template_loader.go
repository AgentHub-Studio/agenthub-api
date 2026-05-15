package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreSessionPersistenceChannelTemplate is a platform-managed preset for one of the
// three §9.1 session persistence channels.
type CoreSessionPersistenceChannelTemplate struct {
	ID              uuid.UUID
	Slug            string
	Label           string
	Description     string
	StorageFormat   string // jsonl|json|db_row
	IsAppendOnly    bool
	IsProjectScoped bool
	IsAlwaysActive  bool
	SortOrder       int
}

// SeedExpectedSessionPersistenceChannelSlugs is the canonical closed set from §9.1.
var SeedExpectedSessionPersistenceChannelSlugs = []string{
	"session_transcripts",
	"global_prompt_history",
	"subagent_sidechains",
}

// SeedExpectedSessionPersistenceChannelRowCount matches the migration INSERT count.
const SeedExpectedSessionPersistenceChannelRowCount = 3

// SeedSessionPersistenceAlwaysActiveSlugs are the channels active in every session.
var SeedSessionPersistenceAlwaysActiveSlugs = []string{
	"session_transcripts",
	"global_prompt_history",
}

// SeedSessionPersistenceProjectScopedSlugs are the channels scoped to a project/session pair.
var SeedSessionPersistenceProjectScopedSlugs = []string{
	"session_transcripts",
	"subagent_sidechains",
}

// SeedSessionPersistenceAppendOnlySlug confirms all three channels are append-only.
const SeedSessionPersistenceAllAppendOnly = true

// CoreSessionPersistenceChannelDefaultTemplateLoader loads channel presets from ah_core.
type CoreSessionPersistenceChannelDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSessionPersistenceChannelDefaultTemplateLoader creates a loader.
func NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool *pgxpool.Pool) *CoreSessionPersistenceChannelDefaultTemplateLoader {
	return &CoreSessionPersistenceChannelDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all channels ordered by sort_order.
func (l *CoreSessionPersistenceChannelDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreSessionPersistenceChannelTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       storage_format, is_append_only, is_project_scoped, is_always_active, sort_order
		  FROM ah_core.session_persistence_channel_template
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.session_persistence_channel_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query session_persistence_channel_template: %w", err)
	}
	defer rows.Close()

	var channels []CoreSessionPersistenceChannelTemplate
	for rows.Next() {
		var t CoreSessionPersistenceChannelTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.StorageFormat, &t.IsAppendOnly, &t.IsProjectScoped, &t.IsAlwaysActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan session_persistence_channel_template: %w", err)
		}
		channels = append(channels, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate session_persistence_channel_template: %w", err)
	}
	return channels, nil
}

// FindBySlug returns one channel template by slug.
func (l *CoreSessionPersistenceChannelDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreSessionPersistenceChannelTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreSessionPersistenceChannelTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreSessionPersistenceChannelTemplate{}, false, nil
}

// LoadAlwaysActive returns only channels active in every session.
func (l *CoreSessionPersistenceChannelDefaultTemplateLoader) LoadAlwaysActive(ctx context.Context) ([]CoreSessionPersistenceChannelTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSessionPersistenceChannelTemplate
	for _, t := range all {
		if t.IsAlwaysActive {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadProjectScoped returns only channels scoped to a project/session pair.
func (l *CoreSessionPersistenceChannelDefaultTemplateLoader) LoadProjectScoped(ctx context.Context) ([]CoreSessionPersistenceChannelTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreSessionPersistenceChannelTemplate
	for _, t := range all {
		if t.IsProjectScoped {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
