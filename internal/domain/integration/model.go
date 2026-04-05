package integration

import (
	"time"

	"github.com/google/uuid"
)

// IntegrationType is the top-level capability category exposed in the simplified admin UX.
type IntegrationType string

const (
	IntegrationTypeHTTPAPI       IntegrationType = "HTTP_API"
	IntegrationTypeDatabaseQuery IntegrationType = "DATABASE_QUERY"
	IntegrationTypeMCP           IntegrationType = "MCP"
)

// IntegrationOrigin describes how the integration entered the catalog.
type IntegrationOrigin string

const (
	IntegrationOriginLegacy    IntegrationOrigin = "legacy"
	IntegrationOriginGenerated IntegrationOrigin = "generated"
	IntegrationOriginManual    IntegrationOrigin = "manual"
)

// SourceKind identifies the legacy source backing a catalog entry.
type SourceKind string

const (
	SourceKindTool       SourceKind = "tool"
	SourceKindDatasource SourceKind = "datasource"
	SourceKindMCP        SourceKind = "mcp_server_config"
	SourceKindVPN        SourceKind = "vpn_resource"
)

// ListFilters contains the supported query filters for the integration catalog.
type ListFilters struct {
	Type    *IntegrationType
	Enabled *bool
	Origin  *IntegrationOrigin
}

// Integration is the normalized catalog entry used by the simplified admin UX.
type Integration struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Type        IntegrationType
	Description string
	Summary     string
	Enabled     bool
	Advanced    bool
	Origin      IntegrationOrigin
	SourceKind  SourceKind
	LegacyPath  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
