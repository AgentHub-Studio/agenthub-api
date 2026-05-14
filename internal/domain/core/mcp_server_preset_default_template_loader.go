package core

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MCPServerPresetDefaultTemplate is one row from ah_core.mcp_server_preset_template.
type MCPServerPresetDefaultTemplate struct {
	ID            string
	Slug          string
	Label         string
	Description   string
	TransportType string
	Command       *string
	Args          []string
	EnvKeys       []string
	BaseURL       *string
	AutoStart     bool
	SortOrder     int
}

// Seed-time canonical constants.

const SeedExpectedMCPPresetRowCount = 6

var SeedExpectedMCPPresetSlugs = []string{
	"filesystem",
	"github",
	"sqlite",
	"brave-search",
	"sequential-thinking",
	"remote-sse",
}

// SeedMCPPresetSlugRE is the kebab-case pattern every slug must satisfy.
var SeedMCPPresetSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SeedMCPStdioPresetSlugs lists presets that use the stdio transport.
var SeedMCPStdioPresetSlugs = []string{
	"filesystem",
	"github",
	"sqlite",
	"brave-search",
	"sequential-thinking",
}

// SeedMCPHTTPPresetSlugs lists presets that use the streamable_http transport.
var SeedMCPHTTPPresetSlugs = []string{
	"remote-sse",
}

// SeedMCPAutoStartSlugs lists presets with auto_start = true.
var SeedMCPAutoStartSlugs = []string{
	"filesystem",
	"github",
	"sqlite",
	"brave-search",
	"sequential-thinking",
}

// CoreMCPServerPresetDefaultTemplateLoader loads MCP preset templates from ah_core.
type CoreMCPServerPresetDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreMCPServerPresetDefaultTemplateLoader constructs a loader backed by pool.
func NewCoreMCPServerPresetDefaultTemplateLoader(pool *pgxpool.Pool) *CoreMCPServerPresetDefaultTemplateLoader {
	return &CoreMCPServerPresetDefaultTemplateLoader{pool: pool}
}

const mcpPresetLoadAllQuery = `
SELECT id, slug, label, description, transport_type, command, args, env_keys, base_url, auto_start, sort_order
FROM ah_core.mcp_server_preset_template
ORDER BY sort_order ASC, slug ASC
`

// LoadAll returns every template ordered by sort_order ASC, slug ASC.
// Returns an empty slice (not an error) when the table does not exist.
func (l *CoreMCPServerPresetDefaultTemplateLoader) LoadAll(ctx context.Context) ([]*MCPServerPresetDefaultTemplate, error) {
	rows, err := l.pool.Query(ctx, mcpPresetLoadAllQuery)
	if err != nil {
		if isUndefinedRelation(err) {
			return []*MCPServerPresetDefaultTemplate{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []*MCPServerPresetDefaultTemplate
	for rows.Next() {
		t := &MCPServerPresetDefaultTemplate{}
		var argsRaw, envKeysRaw []byte
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.TransportType, &t.Command, &argsRaw, &envKeysRaw,
			&t.BaseURL, &t.AutoStart, &t.SortOrder,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(argsRaw, &t.Args); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(envKeysRaw, &t.EnvKeys); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FindBySlug returns the template matching slug, or (nil, false, nil) when not found.
func (l *CoreMCPServerPresetDefaultTemplateLoader) FindBySlug(
	ctx context.Context, slug string,
) (*MCPServerPresetDefaultTemplate, bool, error) {
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

// LoadByTransportType returns templates filtered to a specific transport type.
func (l *CoreMCPServerPresetDefaultTemplateLoader) LoadByTransportType(
	ctx context.Context, transportType string,
) ([]*MCPServerPresetDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*MCPServerPresetDefaultTemplate
	for _, t := range all {
		if t.TransportType == transportType {
			out = append(out, t)
		}
	}
	return out, nil
}

// LoadAutoStart returns only templates with auto_start = true.
func (l *CoreMCPServerPresetDefaultTemplateLoader) LoadAutoStart(
	ctx context.Context,
) ([]*MCPServerPresetDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var out []*MCPServerPresetDefaultTemplate
	for _, t := range all {
		if t.AutoStart {
			out = append(out, t)
		}
	}
	return out, nil
}
