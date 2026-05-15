package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreRule represents a platform-managed behavioural rule from ah_core.rule.
// Rules are short directives injected into agent system prompts to enforce
// universal safety / quality / behavioural / privacy invariants — the
// AgentHub web equivalent of Claude Code's CLAUDE.md hierarchy + managed
// policy files.
//
// Inspired by PDF arXiv:2604.14228v1 Section 7.2 + Section 5.
type CoreRule struct {
	ID                     uuid.UUID
	Name                   string
	Slug                   string
	Description            string
	Content                string
	Category               string // "safety" | "quality" | "behavior" | "privacy"
	Scope                  string // "global" | "tool:<slug>" | "skill:<slug>" | "category:<value>"
	Priority               int
	RequiresAdminToDisable bool
	IsActive               bool
	SortOrder              int
}

// CoreRuleLoader loads platform-managed rules from the global ah_core schema.
// Like other ah_core loaders, it is non-fatal when the schema is missing
// (fresh deployments before the seed migration has run).
type CoreRuleLoader struct {
	pool *pgxpool.Pool
}

// NewCoreRuleLoader creates a CoreRuleLoader backed by the given pool.
func NewCoreRuleLoader(pool *pgxpool.Pool) *CoreRuleLoader {
	return &CoreRuleLoader{pool: pool}
}

// LoadAll returns all active rules from ah_core.rule, ordered by
// (priority DESC, sort_order, slug) so the highest-priority rules
// surface first when injecting into the system prompt.
//
// Returns an empty slice (not an error) when ah_core.rule is missing —
// supports running the API before the seed migration has applied.
func (l *CoreRuleLoader) LoadAll(ctx context.Context) ([]CoreRule, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       content,
		       category, scope, priority,
		       requires_admin_to_disable, is_active, sort_order
		  FROM ah_core.rule
		 WHERE is_active = true
		 ORDER BY priority DESC, sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.rule not accessible, core rules unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query rules: %w", err)
	}
	defer rows.Close()

	var rules []CoreRule
	for rows.Next() {
		var r CoreRule
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Slug, &r.Description, &r.Content,
			&r.Category, &r.Scope, &r.Priority,
			&r.RequiresAdminToDisable, &r.IsActive, &r.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.rule not accessible (post-iter), core rules unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate rules: %w", err)
	}
	return rules, nil
}

// LoadByScope returns the active rules whose scope matches the given
// targets. Always includes "global" rules; appends rules whose scope
// equals one of the supplied targets (e.g. "tool:execute-sql",
// "skill:document-search", "category:research").
func (l *CoreRuleLoader) LoadByScope(ctx context.Context, targets ...string) ([]CoreRule, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{"global": true}
	for _, t := range targets {
		want[t] = true
	}
	var filtered []CoreRule
	for _, r := range all {
		if want[r.Scope] {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// FindBySlug returns the rule with the given slug if present.
func (l *CoreRuleLoader) FindBySlug(ctx context.Context, slug string) (CoreRule, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreRule{}, false, err
	}
	for _, r := range all {
		if r.Slug == slug {
			return r, true, nil
		}
	}
	return CoreRule{}, false, nil
}

// SeedExpectedRuleSlugs is the canonical list of slugs the seed migration
// 000009_seed_rules installs. Tests assert this list matches the actual
// rows so a missing/typo'd seed entry is caught at test time.
//
// Keep this list in sync with migrations/ah_core/000009_seed_rules.up.sql.
var SeedExpectedRuleSlugs = []string{
	// safety (4 — admin-only disable)
	"safety-no-secret-disclosure",
	"safety-confirm-irreversible",
	"safety-decline-illegal",
	"safety-escalate-uncertain",
	// quality (3)
	"quality-cite-sources",
	"quality-acknowledge-uncertainty",
	"quality-prefer-existing-context",
	// behavior (3)
	"behavior-be-concise",
	"behavior-ask-when-ambiguous",
	"behavior-respectful-tone",
	// privacy (3 — admin-only disable)
	"privacy-redact-pii",
	"privacy-minimize-collection",
	"privacy-no-cross-tenant-leak",
}

// SeedExpectedRuleCategories is the closed set of categories the seed uses.
// New categories require updating the admin UI grouping — this list keeps
// the seed and the UI in sync.
var SeedExpectedRuleCategories = []string{
	"safety", "quality", "behavior", "privacy",
}

// SeedAdminOnlyDisableSlugs lists rules tenants cannot disable without
// admin role. Used both for tests and for the runtime authorization gate.
var SeedAdminOnlyDisableSlugs = []string{
	// safety (all 4)
	"safety-no-secret-disclosure",
	"safety-confirm-irreversible",
	"safety-decline-illegal",
	"safety-escalate-uncertain",
	// privacy (all 3)
	"privacy-redact-pii",
	"privacy-minimize-collection",
	"privacy-no-cross-tenant-leak",
}

// Compile-time assertion that the local helper from command_loader.go is
// reused — keeps both loaders in sync on the SQLSTATE handling.
var _ = errors.New // ensure errors import is used (errors.As inside isUndefinedRelation)
var _ = (*pgconn.PgError)(nil)
