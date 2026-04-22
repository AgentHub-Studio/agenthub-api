// Package provider exposes a curated catalog of integration templates
// (GitHub MCP, PostgreSQL, Slack, …) that tenants can instantiate with
// one click via the admin UI. The catalog lives in the public schema
// because the list of supported providers is global — not per-tenant.
package provider

import (
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned when a provider cannot be found by slug.
var ErrNotFound = errors.New("provider: not found")

// Kind is the discriminator that determines how integration.Service
// instantiates the provider (http / database / mcp).
type Kind string

const (
	KindHTTP     Kind = "http"
	KindDatabase Kind = "database"
	KindMCP      Kind = "mcp"
)

// Provider is the domain entity for one catalog entry in provider_template.
type Provider struct {
	Slug         string
	Name         string
	Description  string
	Icon         string
	Category     string
	Kind         Kind
	TemplateJSON json.RawMessage
	IsBuiltin    bool
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
