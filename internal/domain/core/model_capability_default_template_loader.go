package core

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ModelCapabilityDefaultTemplate is one row from ah_core.model_capability_template.
type ModelCapabilityDefaultTemplate struct {
	ModelID          string
	Family           string
	Label            string
	Description      string
	ContextWindow    int
	MaxOutputTokens  int
	RecommendedFor   []string
	SortOrder        int
}

// Seed-time canonical constants.

const SeedExpectedModelCapRowCount = 3

var SeedExpectedModelCapModelIDs = []string{
	"claude-haiku-4-5-20251001",
	"claude-sonnet-4-6",
	"claude-opus-4-7",
}

// SeedModelCapFamilies lists all model families represented in the seed.
var SeedModelCapFamilies = []string{"haiku", "sonnet", "opus"}

// SeedModelCapFamilyRE is the pattern every family name must satisfy.
var SeedModelCapFamilyRE = regexp.MustCompile(`^[a-z]+$`)

// SeedModelCapMaxContextWindow is the context window shared by all seeded models.
const SeedModelCapMaxContextWindow = 200_000

// CoreModelCapabilityDefaultTemplateLoader loads model capability templates from ah_core.
type CoreModelCapabilityDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreModelCapabilityDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreModelCapabilityDefaultTemplateLoader(pool *pgxpool.Pool) *CoreModelCapabilityDefaultTemplateLoader {
	return &CoreModelCapabilityDefaultTemplateLoader{pool: pool}
}

const modelCapLoadAllQuery = `
SELECT model_id, family, label, description, context_window, max_output_tokens, recommended_for, sort_order
FROM ah_core.model_capability_template
ORDER BY sort_order ASC, model_id ASC
`

// LoadAll returns every template ordered by sort_order ASC, model_id ASC.
// Returns an empty slice (not an error) when the table does not exist.
func (l *CoreModelCapabilityDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*ModelCapabilityDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, modelCapLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*ModelCapabilityDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*ModelCapabilityDefaultTemplate
	for rows.Next() {
		t := &ModelCapabilityDefaultTemplate{}
		var recRaw []byte
		if err := rows.Scan(
			&t.ModelID, &t.Family, &t.Label, &t.Description,
			&t.ContextWindow, &t.MaxOutputTokens, &recRaw, &t.SortOrder,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(recRaw, &t.RecommendedFor); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindByModelID returns the template matching modelID, or (nil, false, nil) when not found.
func (l *CoreModelCapabilityDefaultTemplateLoader) FindByModelID(
	ctx context.Context, modelID string,
) (*ModelCapabilityDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, t := range all {
		if t.ModelID == modelID {
			return t, true, nil
		}
	}
	return nil, false, nil
}

// LoadByFamily returns templates filtered to a specific model family (haiku/sonnet/opus).
func (l *CoreModelCapabilityDefaultTemplateLoader) LoadByFamily(
	ctx context.Context, family string,
) ([]*ModelCapabilityDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*ModelCapabilityDefaultTemplate
	for _, t := range all {
		if t.Family == family {
			out = append(out, t)
		}
	}
	return out, nil
}
