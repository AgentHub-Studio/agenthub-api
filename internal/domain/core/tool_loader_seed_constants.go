package core

// SeedExpectedToolSlugs is the canonical list of slugs the seed migration
// 000002_seed_tools installs. Tests assert this list matches actual rows.
// Backfill of integration test for an existing migration — keeps the
// loader contract stable as the migration evolves.
var SeedExpectedToolSlugs = []string{
	// agents (6)
	"core-list-agents",
	"core-get-agent",
	"core-create-agent",
	"core-update-agent",
	"core-delete-agent",
	"core-publish-agent",
	// skills (5)
	"core-list-skills",
	"core-create-skill",
	"core-update-skill",
	"core-delete-skill",
	"core-bind-agent-skills",
	// agent <-> skills (1)
	"core-list-agent-skills",
	// tools (4)
	"core-list-tools",
	"core-create-tool",
	"core-update-tool",
	"core-delete-tool",
	// knowledge bases (4)
	"core-list-kbs",
	"core-create-kb",
	"core-update-kb",
	"core-delete-kb",
	// documents (1)
	"core-upload-document",
	// mcp servers (4)
	"core-list-mcp-servers",
	"core-create-mcp-server",
	"core-update-mcp-server",
	"core-delete-mcp-server",
	// executions (2)
	"core-list-executions",
	"core-get-execution",
	// settings (2)
	"core-get-settings",
	"core-update-settings",
	// import/export (2)
	"core-export-agent",
	"core-import-agent",
}

// SeedExpectedToolType is the only `type` value used by the seed.
// Migration 000002 installs HTTP tools exclusively (proxy REST calls
// to the AgentHub backend with use_caller_token=true).
const SeedExpectedToolType = "HTTP"

// SeedExpectedToolSlugPrefix is the namespace prefix every seed tool
// uses. Custom tenant tools must NOT use this prefix.
const SeedExpectedToolSlugPrefix = "core-"

// SeedExpectedToolConfigKeys is the closed set of top-level keys the
// HTTP tool config JSON uses. Validated against DB shape.
var SeedExpectedToolConfigKeys = []string{
	"url", "method", "useCallerToken", "bodyTemplate",
}

// SeedRequiredToolConfigKeys is the subset of config keys EVERY seed
// tool MUST have (url, method, useCallerToken).
var SeedRequiredToolConfigKeys = []string{
	"url", "method", "useCallerToken",
}
