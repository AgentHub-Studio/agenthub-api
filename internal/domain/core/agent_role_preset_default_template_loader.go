package core

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentRolePresetDefaultTemplate is one row from
// ah_core.agent_role_preset_template.
// Maps Claude Code's built-in subagent types (PDF §8) to AgentHub web roles.
type AgentRolePresetDefaultTemplate struct {
	ID                 string
	Slug               string
	Label              string
	Description        string
	SourceSubagentType string
	DefaultContextMode string
	DefaultEffortLevel string
	AllowedTools       []string
	DisallowedTools    []string
	PermissionMode     string
	RecommendedFor     []string
	SortOrder          int
}

const SeedExpectedAgentRolePresetRowCount = 5

var SeedExpectedAgentRolePresetSlugs = []string{
	"general-assistant",
	"read-researcher",
	"planner",
	"validator",
	"documentation-guide",
}

// SeedAgentRolePresetSlugRE is the kebab-case pattern every slug must satisfy.
var SeedAgentRolePresetSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SeedAgentRoleDefaultSlug is the slug used when no role is specified.
const SeedAgentRoleDefaultSlug = "general-assistant"

// SeedAgentRolePlannerSlug is the slug for the plan-mode preset.
const SeedAgentRolePlannerSlug = "planner"

// SeedAgentRoleReadOnlySlug is the slug for the read-only research preset.
const SeedAgentRoleReadOnlySlug = "read-researcher"

// SeedAgentRoleSourceTypes lists the Claude Code built-in types covered by the seed.
var SeedAgentRoleSourceTypes = []string{
	"general-purpose", "explore", "plan", "verification", "claude-code-guide",
}

// CoreAgentRolePresetDefaultTemplateLoader loads role preset templates from ah_core.
type CoreAgentRolePresetDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreAgentRolePresetDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreAgentRolePresetDefaultTemplateLoader(pool *pgxpool.Pool) *CoreAgentRolePresetDefaultTemplateLoader {
	return &CoreAgentRolePresetDefaultTemplateLoader{pool: pool}
}

const agentRoleLoadAllQuery = `
SELECT id, slug, label, description, source_subagent_type,
       default_context_mode, default_effort_level,
       allowed_tools, disallowed_tools, permission_mode, recommended_for, sort_order
FROM ah_core.agent_role_preset_template
ORDER BY sort_order ASC, slug ASC
`

// LoadAll returns every template ordered by sort_order ASC, slug ASC.
// Returns an empty slice (not an error) when the table does not exist.
func (l *CoreAgentRolePresetDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*AgentRolePresetDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, agentRoleLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*AgentRolePresetDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*AgentRolePresetDefaultTemplate
	for rows.Next() {
		t := &AgentRolePresetDefaultTemplate{}
		var allowedRaw, disallowedRaw, recRaw []byte
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description, &t.SourceSubagentType,
			&t.DefaultContextMode, &t.DefaultEffortLevel,
			&allowedRaw, &disallowedRaw, &t.PermissionMode, &recRaw, &t.SortOrder,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(allowedRaw, &t.AllowedTools); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(disallowedRaw, &t.DisallowedTools); err != nil {
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
func (l *CoreAgentRolePresetDefaultTemplateLoader) FindBySlug(
	ctx context.Context, slug string,
) (*AgentRolePresetDefaultTemplate, bool, error) {
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

// LoadByPermissionMode returns templates filtered to a specific permission_mode.
func (l *CoreAgentRolePresetDefaultTemplateLoader) LoadByPermissionMode(
	ctx context.Context, mode string,
) ([]*AgentRolePresetDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*AgentRolePresetDefaultTemplate
	for _, t := range all {
		if t.PermissionMode == mode {
			out = append(out, t)
		}
	}
	return out, nil
}

// LoadByContextMode returns templates filtered to a specific context mode (inline/fork).
func (l *CoreAgentRolePresetDefaultTemplateLoader) LoadByContextMode(
	ctx context.Context, mode string,
) ([]*AgentRolePresetDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*AgentRolePresetDefaultTemplate
	for _, t := range all {
		if t.DefaultContextMode == mode {
			out = append(out, t)
		}
	}
	return out, nil
}
