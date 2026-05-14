package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreContextAssemblySourceTemplate is a platform-managed preset for one of the
// nine §7.1 context window assembly sources.
type CoreContextAssemblySourceTemplate struct {
	ID               uuid.UUID
	Slug             string
	Label            string
	Description      string
	SourceOrder      int    // 1–9, canonical assembly order per §7.1
	Domain           string // prompt_construction|platform_context|instruction_files|memory|tool_definitions|conversation
	IsMemoized       bool
	IsAsynchronous   bool
	IsAlwaysIncluded bool
	SortOrder        int
}

// SeedExpectedContextAssemblySourceSlugs is the canonical closed set from §7.1.
var SeedExpectedContextAssemblySourceSlugs = []string{
	"system_prompt",
	"environment_info",
	"claude_md_hierarchy",
	"path_scoped_rules",
	"auto_memory",
	"tool_metadata",
	"conversation_history",
	"tool_results",
	"compact_summaries",
}

// SeedExpectedContextAssemblySourceRowCount matches the migration INSERT count.
const SeedExpectedContextAssemblySourceRowCount = 9

// SeedContextAssemblySourceDomains is the closed set of domain values.
var SeedContextAssemblySourceDomains = []string{
	"prompt_construction", "platform_context", "instruction_files",
	"memory", "tool_definitions", "conversation",
}

// SeedContextAssemblyAlwaysIncludedSlugs are the sources present in every turn.
var SeedContextAssemblyAlwaysIncludedSlugs = []string{
	"system_prompt", "environment_info", "claude_md_hierarchy",
	"tool_metadata", "conversation_history", "tool_results",
}

// SeedContextAssemblyAsyncSlugs are the sources prefetched asynchronously.
var SeedContextAssemblyAsyncSlugs = []string{"auto_memory", "tool_metadata"}

// SeedContextAssemblyMemoizedSlugs are the sources cached for the session lifetime.
var SeedContextAssemblyMemoizedSlugs = []string{"environment_info", "claude_md_hierarchy"}

// CoreContextAssemblySourceDefaultTemplateLoader loads source presets from ah_core.
type CoreContextAssemblySourceDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreContextAssemblySourceDefaultTemplateLoader creates a loader.
func NewCoreContextAssemblySourceDefaultTemplateLoader(pool *pgxpool.Pool) *CoreContextAssemblySourceDefaultTemplateLoader {
	return &CoreContextAssemblySourceDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all sources ordered by source_order.
func (l *CoreContextAssemblySourceDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreContextAssemblySourceTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       source_order, domain, is_memoized, is_asynchronous, is_always_included, sort_order
		  FROM ah_core.context_assembly_source_template
		 ORDER BY source_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.context_assembly_source_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query context_assembly_source_template: %w", err)
	}
	defer rows.Close()

	var sources []CoreContextAssemblySourceTemplate
	for rows.Next() {
		var t CoreContextAssemblySourceTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.SourceOrder, &t.Domain, &t.IsMemoized, &t.IsAsynchronous, &t.IsAlwaysIncluded, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan context_assembly_source_template: %w", err)
		}
		sources = append(sources, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate context_assembly_source_template: %w", err)
	}
	return sources, nil
}

// FindBySlug returns one source template by slug.
func (l *CoreContextAssemblySourceDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreContextAssemblySourceTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreContextAssemblySourceTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreContextAssemblySourceTemplate{}, false, nil
}

// LoadAlwaysIncluded returns only sources present in every turn.
func (l *CoreContextAssemblySourceDefaultTemplateLoader) LoadAlwaysIncluded(ctx context.Context) ([]CoreContextAssemblySourceTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextAssemblySourceTemplate
	for _, t := range all {
		if t.IsAlwaysIncluded {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadAsync returns only sources prefetched asynchronously.
func (l *CoreContextAssemblySourceDefaultTemplateLoader) LoadAsync(ctx context.Context) ([]CoreContextAssemblySourceTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextAssemblySourceTemplate
	for _, t := range all {
		if t.IsAsynchronous {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadInDomain returns sources belonging to the given domain, in source_order.
func (l *CoreContextAssemblySourceDefaultTemplateLoader) LoadInDomain(ctx context.Context, domain string) ([]CoreContextAssemblySourceTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextAssemblySourceTemplate
	for _, t := range all {
		if t.Domain == domain {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
