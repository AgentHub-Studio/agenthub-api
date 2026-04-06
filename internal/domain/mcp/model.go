package mcp

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an MCP server config cannot be found.
var ErrNotFound = errors.New("mcp: not found")

// McpServerConfig is the domain entity (table: mcp_server_config).
// No tenant_id field — isolation is provided via schema search_path.
type McpServerConfig struct {
	ID               uuid.UUID         `db:"id"`
	Name             string            `db:"name"`
	TransportType    string            `db:"transport_type"`
	HTTPBaseURL      *string           `db:"http_base_url"`
	Command          *string           `db:"command"`
	Args             []string          `db:"args"`
	Env               map[string]string `db:"env"`
	OAuthCredentialID *uuid.UUID        `db:"oauth_credential_id"`
	AutoStart         bool              `db:"auto_start"`
	Enabled          bool              `db:"enabled"`
	CreatedAt        time.Time         `db:"created_at"`
	UpdatedAt        time.Time         `db:"updated_at"`
}

// McpServerConfigResponse is the DTO returned by the API.
// OAuthClientSecret is intentionally omitted for security.
type McpServerConfigResponse struct {
	ID            uuid.UUID         `json:"id"`
	Name          string            `json:"name"`
	TransportType string            `json:"transportType"`
	HTTPBaseURL   *string           `json:"httpBaseUrl,omitempty"`
	Command       *string           `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	OAuthCredentialID *uuid.UUID        `json:"oauthCredentialId,omitempty"`
	AutoStart         bool              `json:"autoStart"`
	Enabled       bool              `json:"enabled"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// ResponseFrom maps a McpServerConfig entity to a McpServerConfigResponse DTO.
func ResponseFrom(c McpServerConfig) McpServerConfigResponse {
	return McpServerConfigResponse{
		ID:            c.ID,
		Name:          c.Name,
		TransportType: c.TransportType,
		HTTPBaseURL:   c.HTTPBaseURL,
		Command:       c.Command,
		Args:              c.Args,
		Env:               c.Env,
		OAuthCredentialID: c.OAuthCredentialID,
		AutoStart:         c.AutoStart,
		Enabled:       c.Enabled,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

// CreateRequest is the payload for creating an MCP server config.
type CreateRequest struct {
	Name              string            `json:"name"`
	TransportType     string            `json:"transportType"`
	HTTPBaseURL       *string           `json:"httpBaseUrl"`
	Command           *string           `json:"command"`
	Args              []string          `json:"args"`
	Env               map[string]string `json:"env"`
	OAuthCredentialID *uuid.UUID        `json:"oauthCredentialId"`
	AutoStart         bool              `json:"autoStart"`
	Enabled           bool              `json:"enabled"`
}

// UpdateRequest is the payload for updating an MCP server config (all fields optional).
type UpdateRequest struct {
	Name              *string            `json:"name"`
	TransportType     *string            `json:"transportType"`
	HTTPBaseURL       *string            `json:"httpBaseUrl"`
	Command           *string            `json:"command"`
	Args              *[]string          `json:"args"`
	Env               *map[string]string `json:"env"`
	OAuthCredentialID *uuid.UUID         `json:"oauthCredentialId"`
	AutoStart         *bool              `json:"autoStart"`
	Enabled           *bool              `json:"enabled"`
}
