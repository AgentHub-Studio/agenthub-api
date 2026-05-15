package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreSkill represents a platform-managed skill from ah_core.skill.
// Skills are abstract capabilities (PDF Section 6.1) backed by one or
// more tools. Bindings live in ah_core.skill_tool.
type CoreSkill struct {
	ID                     uuid.UUID
	Name                   string
	Slug                   string
	Description            string
	Instructions           string
	Category               string
	DisableModelInvocation bool
	ContextMode            string // inline | reference | dynamic
	WhenToUse              string
}

// CoreSkillToolBinding represents a skill→tool binding from
// ah_core.skill_tool.
type CoreSkillToolBinding struct {
	ID       uuid.UUID
	SkillID  uuid.UUID
	ToolID   uuid.UUID
	Priority int
	IsActive bool
}

// CoreSkillLoader loads platform-managed skills from ah_core.
// Like other core loaders, non-fatal when schema missing.
type CoreSkillLoader struct {
	pool *pgxpool.Pool
}

// NewCoreSkillLoader creates a CoreSkillLoader backed by the given pool.
func NewCoreSkillLoader(pool *pgxpool.Pool) *CoreSkillLoader {
	return &CoreSkillLoader{pool: pool}
}

// LoadAll returns all skills from ah_core.skill, ordered by name.
func (l *CoreSkillLoader) LoadAll(ctx context.Context) ([]CoreSkill, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       COALESCE(instructions, '') AS instructions,
		       category, disable_model_invocation, context_mode,
		       COALESCE(when_to_use, '') AS when_to_use
		  FROM ah_core.skill
		 ORDER BY name`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.skill not accessible, core skills unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query skills: %w", err)
	}
	defer rows.Close()

	var skills []CoreSkill
	for rows.Next() {
		var s CoreSkill
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Slug, &s.Description, &s.Instructions,
			&s.Category, &s.DisableModelInvocation, &s.ContextMode, &s.WhenToUse,
		); err != nil {
			return nil, fmt.Errorf("core: scan skill: %w", err)
		}
		skills = append(skills, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.skill not accessible (post-iter)", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate skills: %w", err)
	}
	return skills, nil
}

// FindBySlug returns one skill by slug.
func (l *CoreSkillLoader) FindBySlug(ctx context.Context, slug string) (CoreSkill, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreSkill{}, false, err
	}
	for _, s := range all {
		if s.Slug == slug {
			return s, true, nil
		}
	}
	return CoreSkill{}, false, nil
}

// LoadToolBindings returns all skill→tool bindings ordered by skill_id, priority.
// Used to materialize the skill catalog into agent tool listings.
func (l *CoreSkillLoader) LoadToolBindings(ctx context.Context) ([]CoreSkillToolBinding, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, skill_id, tool_id, priority, is_active
		  FROM ah_core.skill_tool
		 WHERE is_active = true
		 ORDER BY skill_id, priority, id`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.skill_tool not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query skill_tool: %w", err)
	}
	defer rows.Close()

	var bindings []CoreSkillToolBinding
	for rows.Next() {
		var b CoreSkillToolBinding
		if err := rows.Scan(&b.ID, &b.SkillID, &b.ToolID, &b.Priority, &b.IsActive); err != nil {
			return nil, fmt.Errorf("core: scan skill_tool: %w", err)
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate skill_tool: %w", err)
	}
	return bindings, nil
}

// SeedExpectedSkillSlugs is the canonical list of slugs the seed
// migration 000003_seed_skills installs.
var SeedExpectedSkillSlugs = []string{
	"core-agents-management",
	"core-skills-management",
	"core-tools-management",
	"core-kb-management",
	"core-execution-management",
	"core-mcp-management",
	"core-platform-settings",
}

// SeedExpectedSkillCategory is the only category seeded skills use.
const SeedExpectedSkillCategory = "platform"

// SeedExpectedSkillSlugPrefix — namespace contract for platform skills.
const SeedExpectedSkillSlugPrefix = "core-"

// SeedExpectedSkillContextMode is the default context mode for seeded skills.
// `inline` = instructions injected into system prompt (PDF Section 6.1).
const SeedExpectedSkillContextMode = "inline"

// SeedExpectedSkillBindingsCount is the total number of skill→tool
// bindings the seed installs (sum across all 7 skills).
//
// Bindings per skill (from migration 000003):
//   agents-management    → 10 (list/get/create/update/delete/publish + bind-agent-skills + list-agent-skills + export + import)
//   skills-management    → 4  (list/create/update/delete)
//   tools-management     → 4  (list/create/update/delete)
//   kb-management        → 5  (list/create/update/delete + upload-document)
//   execution-management → 2  (list/get)
//   mcp-management       → 4  (list/create/update/delete)
//   platform-settings    → 2  (get/update)
//
// Sum: 10+4+4+5+2+4+2 = 31. Integration test asserts.
const SeedExpectedSkillBindingsCount = 31
