package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreAgentCapabilityTierTemplate represents a platform-managed agent capability
// tier. Tiers bundle permission mode, tool access, context budget, and governance
// level. Inspired by §13 (arXiv:2604.14228v1): "Managed Agents design virtualizes
// session, harness, and sandbox into independently replaceable interfaces."
type CoreAgentCapabilityTierTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Label                    string
	Description              string
	PermissionMode           string  // default / plan / auto / bypassPermissions
	ToolAccessLevel          string  // read-only / standard / full / none
	ContextWindowFraction    float64 // 0.0–1.0
	MaxToolCallsPerTurn      int     // 0 = unlimited
	AllowsBackgroundRun      bool
	RequiresHumanCheckpoint  bool
	GovernanceLevel          string // none / standard / strict
	DesignValueProfile       string // human_authority / safety / reliability / capability / adaptability
	RecommendedFor           []string
	SortOrder                int
}

// SeedExpectedCapabilityTierSlugs is the canonical closed set.
var SeedExpectedCapabilityTierSlugs = []string{
	"read-only",
	"standard",
	"background",
	"governed",
	"autonomous",
}

// SeedExpectedCapabilityTierRowCount matches the migration INSERT count.
const SeedExpectedCapabilityTierRowCount = 5

// SeedCapabilityTierPermissionModes is the closed set of permission_mode values.
var SeedCapabilityTierPermissionModes = []string{"default", "bypassPermissions"}

// SeedCapabilityTierToolAccessLevels is the closed set of tool_access_level values.
var SeedCapabilityTierToolAccessLevels = []string{"read-only", "standard", "full"}

// SeedCapabilityTierGovernanceLevels is the closed set of governance_level values.
var SeedCapabilityTierGovernanceLevels = []string{"none", "standard", "strict"}

// SeedCapabilityTierDefaultSlug is the preset used when no custom tier is set.
const SeedCapabilityTierDefaultSlug = "standard"

// SeedCapabilityTierGovernedSlug requires human checkpoint (regulated environments).
const SeedCapabilityTierGovernedSlug = "governed"

// SeedCapabilityTierReadOnlySlug is the safest tier (research/documentation).
const SeedCapabilityTierReadOnlySlug = "read-only"

// SeedCapabilityTierSlugRE validates kebab-case slug format.
var SeedCapabilityTierSlugRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// CoreAgentCapabilityTierDefaultTemplateLoader loads tiers from ah_core.
type CoreAgentCapabilityTierDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreAgentCapabilityTierDefaultTemplateLoader creates a loader.
func NewCoreAgentCapabilityTierDefaultTemplateLoader(pool *pgxpool.Pool) *CoreAgentCapabilityTierDefaultTemplateLoader {
	return &CoreAgentCapabilityTierDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all tiers ordered by sort_order, slug.
func (l *CoreAgentCapabilityTierDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreAgentCapabilityTierTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description, permission_mode, tool_access_level,
		       context_window_fraction, max_tool_calls_per_turn,
		       allows_background_run, requires_human_checkpoint,
		       governance_level, design_value_profile,
		       recommended_for, sort_order
		  FROM ah_core.agent_capability_tier_template
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.agent_capability_tier_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query agent_capability_tier_template: %w", err)
	}
	defer rows.Close()

	var tiers []CoreAgentCapabilityTierTemplate
	for rows.Next() {
		var t CoreAgentCapabilityTierTemplate
		var recommendedForRaw []byte
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.PermissionMode, &t.ToolAccessLevel,
			&t.ContextWindowFraction, &t.MaxToolCallsPerTurn,
			&t.AllowsBackgroundRun, &t.RequiresHumanCheckpoint,
			&t.GovernanceLevel, &t.DesignValueProfile,
			&recommendedForRaw, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan agent_capability_tier_template: %w", err)
		}
		if len(recommendedForRaw) > 0 {
			if err := json.Unmarshal(recommendedForRaw, &t.RecommendedFor); err != nil {
				return nil, fmt.Errorf("core: unmarshal recommended_for: %w", err)
			}
		}
		tiers = append(tiers, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate agent_capability_tier_template: %w", err)
	}
	return tiers, nil
}

// FindBySlug returns one tier by slug.
func (l *CoreAgentCapabilityTierDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreAgentCapabilityTierTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreAgentCapabilityTierTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreAgentCapabilityTierTemplate{}, false, nil
}

// LoadByGovernanceLevel returns tiers with the given governance level.
func (l *CoreAgentCapabilityTierDefaultTemplateLoader) LoadByGovernanceLevel(ctx context.Context, level string) ([]CoreAgentCapabilityTierTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreAgentCapabilityTierTemplate
	for _, t := range all {
		if t.GovernanceLevel == level {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadBackgroundCapable returns tiers that allow background execution.
func (l *CoreAgentCapabilityTierDefaultTemplateLoader) LoadBackgroundCapable(ctx context.Context) ([]CoreAgentCapabilityTierTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreAgentCapabilityTierTemplate
	for _, t := range all {
		if t.AllowsBackgroundRun {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
