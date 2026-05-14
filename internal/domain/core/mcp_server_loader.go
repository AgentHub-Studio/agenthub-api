package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreMCPServer represents a platform-managed MCP server CATALOG entry from
// ah_core.mcp_server. Catalog entries are NOT active per-tenant configs —
// tenants enable a catalog entry by inserting a tenant-scoped row in their
// own `mcp_server_config` table (see CLAUDE.md MCP section).
//
// AgentHub is predominantly WEB → only HTTP-transport servers are catalogued.
// STDIO servers require local subprocess and are NOT seeded.
//
// Inspired by PDF arXiv:2604.14228v1 Section 6.1 (MCP servers as one of 4
// extension mechanisms) + Anthropic MCP reference catalog.
type CoreMCPServer struct {
	ID               uuid.UUID
	Name             string
	Slug             string
	Description      string
	TransportType    string // http (seed catalog only) | stdio
	HTTPBaseURL      string
	Category         string // search / development / monitoring / collaboration / data / utility / ai
	IconSlug         string
	RequiresAuth     bool
	AuthType         string // none / api_key / bearer_token / oauth2 / basic
	Vendor           string
	DocumentationURL string
	IsOfficial       bool
	IsActive         bool
	SortOrder        int
}

// CoreMCPServerLoader loads platform-managed MCP server catalog entries.
// Like other core loaders, non-fatal when schema missing.
type CoreMCPServerLoader struct {
	pool *pgxpool.Pool
}

// NewCoreMCPServerLoader creates a CoreMCPServerLoader backed by the given
// pool.
func NewCoreMCPServerLoader(pool *pgxpool.Pool) *CoreMCPServerLoader {
	return &CoreMCPServerLoader{pool: pool}
}

// LoadAll returns all active MCP server catalog entries, ordered by
// sort_order then slug.
func (l *CoreMCPServerLoader) LoadAll(ctx context.Context) ([]CoreMCPServer, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       transport_type,
		       COALESCE(http_base_url, '') AS http_base_url,
		       category,
		       COALESCE(icon_slug, '') AS icon_slug,
		       requires_auth, auth_type, vendor,
		       COALESCE(documentation_url, '') AS documentation_url,
		       is_official, is_active, sort_order
		  FROM ah_core.mcp_server
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.mcp_server not accessible, core MCP catalog unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query mcp servers: %w", err)
	}
	defer rows.Close()

	var servers []CoreMCPServer
	for rows.Next() {
		var s CoreMCPServer
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Slug, &s.Description,
			&s.TransportType, &s.HTTPBaseURL, &s.Category, &s.IconSlug,
			&s.RequiresAuth, &s.AuthType, &s.Vendor, &s.DocumentationURL,
			&s.IsOfficial, &s.IsActive, &s.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan mcp server: %w", err)
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.mcp_server not accessible (post-iter)", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate mcp servers: %w", err)
	}
	return servers, nil
}

// FindBySlug returns one server by slug.
func (l *CoreMCPServerLoader) FindBySlug(ctx context.Context, slug string) (CoreMCPServer, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreMCPServer{}, false, err
	}
	for _, s := range all {
		if s.Slug == slug {
			return s, true, nil
		}
	}
	return CoreMCPServer{}, false, nil
}

// LoadByCategory returns active servers in a specific category, preserving
// sort order. Used by the UI picker to render category-grouped lists.
func (l *CoreMCPServerLoader) LoadByCategory(ctx context.Context, category string) ([]CoreMCPServer, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMCPServer
	for _, s := range all {
		if s.Category == category {
			matched = append(matched, s)
		}
	}
	return matched, nil
}

// SeedExpectedMCPServerSlugs is the canonical list of slugs the seed
// migration 000012_seed_mcp_servers installs.
var SeedExpectedMCPServerSlugs = []string{
	// search (2)
	"brave-search",
	"web-fetch",
	// development (3)
	"github",
	"gitlab",
	"postgres-readonly",
	// monitoring (1)
	"sentry",
	// collaboration (1)
	"slack",
	// utility (2)
	"memory",
	"time",
}

// SeedExpectedMCPServerCategories is the closed set of categories the
// seed uses. UI picker filters by category.
var SeedExpectedMCPServerCategories = []string{
	"search",
	"development",
	"monitoring",
	"collaboration",
	"utility",
}

// SeedExpectedMCPServerAuthTypes is the closed set of auth_type values
// the seed uses. UI shows the matching credential picker.
var SeedExpectedMCPServerAuthTypes = []string{
	"none",
	"api_key",
	"bearer_token",
	"oauth2",
	"basic",
}

// SeedExpectedMCPServerTransports is the closed set of transport_type
// values the seed uses. Web-product seed → http only.
var SeedExpectedMCPServerTransports = []string{
	"http",
}

// SeedAuthRequiredMCPServerSlugs lists the catalog entries that require
// credentials before any tool/resource call.
var SeedAuthRequiredMCPServerSlugs = []string{
	"brave-search",
	"github",
	"gitlab",
	"postgres-readonly",
	"sentry",
	"slack",
}
