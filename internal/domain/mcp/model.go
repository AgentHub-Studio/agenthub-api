package mcp

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an MCP server config cannot be found.
var ErrNotFound = errors.New("mcp: not found")

// ErrDuplicateName é retornado quando POST tenta criar mcp-server-config
// com nome já existente. Mapeia para 409.
var ErrDuplicateName = errors.New("mcp: name already exists")

// ErrRedirectURLNotAllowed prevents OAuth authorization codes from being sent
// to a redirect URI outside the deployment-owned frontend origins.
var ErrRedirectURLNotAllowed = errors.New("mcp: redirect URL is not allowed")

// McpServerConfig is the domain entity (table: mcp_server_config).
// No tenant_id field — isolation is provided via schema search_path.
type McpServerConfig struct {
	ID                uuid.UUID         `db:"id"`
	Name              string            `db:"name"`
	TransportType     string            `db:"transport_type"`
	HTTPBaseURL       *string           `db:"http_base_url"`
	Command           *string           `db:"command"`
	Args              []string          `db:"args"`
	Env               map[string]string `db:"env"`
	OAuthCredentialID *uuid.UUID        `db:"oauth_credential_id"`
	AutoStart         bool              `db:"auto_start"`
	Enabled           bool              `db:"enabled"`
	CreatedAt         time.Time         `db:"created_at"`
	UpdatedAt         time.Time         `db:"updated_at"`
}

// McpServerConfigResponse is the DTO returned by the API.
// OAuthClientSecret is intentionally omitted for security.
type McpServerConfigResponse struct {
	ID                uuid.UUID         `json:"id"`
	Name              string            `json:"name"`
	TransportType     string            `json:"transportType"`
	HTTPBaseURL       *string           `json:"httpBaseUrl,omitempty"`
	Command           *string           `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	OAuthCredentialID *uuid.UUID        `json:"oauthCredentialId,omitempty"`
	AutoStart         bool              `json:"autoStart"`
	Enabled           bool              `json:"enabled"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

// McpServerConfigBootstrapResponse is returned only to the mcp-client-runtime.
// It includes runtime-only OAuth secrets and is protected by service-account roles.
type McpServerConfigBootstrapResponse struct {
	ID                uuid.UUID         `json:"id"`
	Name              string            `json:"name"`
	TransportType     string            `json:"transportType"`
	HTTPBaseURL       string            `json:"httpBaseUrl,omitempty"`
	Command           string            `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	OAuthTokenURL     string            `json:"oauthTokenUrl,omitempty"`
	OAuthClientID     string            `json:"oauthClientId,omitempty"`
	OAuthClientSecret string            `json:"oauthClientSecret,omitempty"`
	OAuthScopes       []string          `json:"oauthScopes,omitempty"`
	OAuthBearerToken  string            `json:"oauthBearerToken,omitempty"`
	OAuthRefreshToken string            `json:"oauthRefreshToken,omitempty"`
	AutoStart         bool              `json:"autoStart"`
	Enabled           bool              `json:"enabled"`
}

// ResponseFrom maps a McpServerConfig entity to a McpServerConfigResponse DTO.
func ResponseFrom(c McpServerConfig) McpServerConfigResponse {
	resp := McpServerConfigResponse(c)
	resp.Env = sanitizeEnv(c.Env)
	return resp
}

func sanitizeEnv(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		if isSensitiveEnvName(key) {
			out[key] = "***"
			continue
		}
		out[key] = redactSensitiveEnvURL(value)
	}
	return out
}

func redactSensitiveEnvURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return value
	}

	changed := false
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), "***")
			changed = true
		}
	}

	query := parsed.Query()
	for key, values := range query {
		if !isSensitiveEnvName(key) {
			continue
		}
		for i := range values {
			values[i] = "***"
		}
		query[key] = values
		changed = true
	}
	if changed {
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return value
}

func isSensitiveEnvName(name string) bool {
	normalized := normalizeEnvName(name)
	for _, marker := range []string{"apikey", "accesskey", "privatekey", "secretkey"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	for _, suffix := range []string{"secret", "password", "token", "credential", "credentials", "authorization"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}

	segments := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for _, segment := range segments {
		switch segment {
		case "secret", "password", "token", "credential", "credentials", "authorization":
			return true
		}
	}
	return false
}

func normalizeEnvName(name string) string {
	replacer := strings.NewReplacer("_", "", "-", "", ".", "")
	return replacer.Replace(strings.ToLower(name))
}

// BootstrapResponseFrom maps an MCP config to the runtime bootstrap DTO.
func BootstrapResponseFrom(c McpServerConfig) McpServerConfigBootstrapResponse {
	resp := McpServerConfigBootstrapResponse{
		ID:            c.ID,
		Name:          c.Name,
		TransportType: c.TransportType,
		Args:          c.Args,
		Env:           c.Env,
		AutoStart:     c.AutoStart,
		Enabled:       c.Enabled,
	}
	if c.HTTPBaseURL != nil {
		resp.HTTPBaseURL = *c.HTTPBaseURL
	}
	if c.Command != nil {
		resp.Command = *c.Command
	}
	return resp
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

// AuthStatusResponse indicates if the MCP server is authenticated.
type AuthStatusResponse struct {
	Authenticated bool          `json:"authenticated"`
	Metadata      *AuthMetadata `json:"metadata,omitempty"`
}

// AuthMetadata contains discovered OAuth2 metadata from the MCP server.
type AuthMetadata struct {
	ResourceMetadataURL string   `json:"resource_metadata_url,omitempty"`
	AuthorizationURL    string   `json:"authorization_url,omitempty"`
	TokenURL            string   `json:"token_url,omitempty"`
	RegistrationURL     string   `json:"registration_url,omitempty"`
	Issuer              string   `json:"issuer,omitempty"`
	ScopesSupported     []string `json:"scopes_supported,omitempty"`
	ClientID            string   `json:"client_id,omitempty"`
}

// ConnectURLResponse provides the URL to initiate OAuth flow.
type ConnectURLResponse struct {
	URL string `json:"url"`
}

// ToolResponse represents a tool exposed by an MCP server.
type ToolResponse struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}
