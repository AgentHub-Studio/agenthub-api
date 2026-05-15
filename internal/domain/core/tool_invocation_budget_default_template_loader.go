package core

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ToolInvocationBudgetDefaultTemplate is one row from
// ah_core.tool_invocation_budget_template.
type ToolInvocationBudgetDefaultTemplate struct {
	ID           string
	Slug         string
	Label        string
	Description  string
	TotalCap     int
	Policy       string
	CategoryCaps map[string]int
	SortOrder    int
}

// Seed-time canonical constants.

const SeedExpectedTIBTemplateRowCount = 6

var SeedExpectedTIBTemplateSlugs = []string{
	"unlimited",
	"interactive-standard",
	"batch-processing",
	"compliance-audit",
	"cost-controlled",
	"research-assistant",
}

// SeedTIBTemplateSlugRE is the kebab-case pattern every slug must satisfy.
var SeedTIBTemplateSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SeedTIBTemplatePolicies lists all policies present in the seed.
var SeedTIBTemplatePolicies = []string{"deny", "warn", "report"}

// SeedTIBReadOnlySlug is the slug of the compliance-audit (zero mutate cap) template.
const SeedTIBReadOnlySlug = "compliance-audit"

// SeedTIBUnlimitedSlug is the slug of the unlimited template.
const SeedTIBUnlimitedSlug = "unlimited"

// CoreToolInvocationBudgetDefaultTemplateLoader loads budget templates from ah_core.
type CoreToolInvocationBudgetDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreToolInvocationBudgetDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreToolInvocationBudgetDefaultTemplateLoader(pool *pgxpool.Pool) *CoreToolInvocationBudgetDefaultTemplateLoader {
	return &CoreToolInvocationBudgetDefaultTemplateLoader{pool: pool}
}

const tibLoadAllQuery = `
SELECT id, slug, label, description, total_cap, policy, category_caps, sort_order
FROM ah_core.tool_invocation_budget_template
ORDER BY sort_order ASC, slug ASC
`

// LoadAll returns every template ordered by sort_order ASC, slug ASC.
// Returns an empty slice (not an error) when the table does not exist.
func (l *CoreToolInvocationBudgetDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*ToolInvocationBudgetDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, tibLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*ToolInvocationBudgetDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*ToolInvocationBudgetDefaultTemplate
	for rows.Next() {
		t := &ToolInvocationBudgetDefaultTemplate{}
		var catRaw []byte
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.TotalCap, &t.Policy, &catRaw, &t.SortOrder,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(catRaw, &t.CategoryCaps); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindBySlug returns the template matching slug, or (nil, false, nil) when not found.
func (l *CoreToolInvocationBudgetDefaultTemplateLoader) FindBySlug(
	ctx context.Context, slug string,
) (*ToolInvocationBudgetDefaultTemplate, bool, error) {
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

// LoadByPolicy returns templates filtered to a specific policy (deny/warn/report).
func (l *CoreToolInvocationBudgetDefaultTemplateLoader) LoadByPolicy(
	ctx context.Context, policy string,
) ([]*ToolInvocationBudgetDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*ToolInvocationBudgetDefaultTemplate
	for _, t := range all {
		if t.Policy == policy {
			out = append(out, t)
		}
	}
	return out, nil
}
