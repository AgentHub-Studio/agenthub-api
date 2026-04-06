package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// oauthService defines methods needed from oauth domain.
type oauthService interface {
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (oauth.OAuthCredential, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req oauth.CreateRequest) (oauth.OAuthCredential, error)
	GeneratePKCE() (verifier string, challenge string)
}

// Service provides business logic for McpServerConfig operations.
type Service struct {
	repo     Repository
	oauthSvc oauthService
	// Discovery from runtime
	mcpRuntimeURL string
}

// mcpRuntimeClient defines methods to query the MCP runtime.
type mcpRuntimeClient interface {
	GetServerStatus(ctx context.Context, name string) (*McpRuntimeStatus, error)
}

// McpRuntimeStatus matches the response from mcp-client-runtime /servers/:name/status
type McpRuntimeStatus struct {
	Name          string        `json:"Name"`
	Status        string        `json:"Status"`
	AuthMetadata  *AuthMetadata `json:"AuthMetadata,omitempty"`
}

// NewService creates a new Service backed by the given Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// WithRuntimeURL attaches the MCP runtime URL to the service.
func (s *Service) WithRuntimeURL(url string) *Service {
	s.mcpRuntimeURL = url
	return s
}

// WithOAuthService attaches the oauth service to the MCP service.
func (s *Service) WithOAuthService(oauthSvc oauthService) *Service {
	s.oauthSvc = oauthSvc
	return s
}

// Repository returns the underlying repository.
func (s *Service) Repository() Repository {
	return s.repo
}

// List returns all MCP server configs for the tenant.
func (s *Service) List(ctx context.Context) ([]McpServerConfigResponse, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp service: list: %w", err)
	}

	responses := make([]McpServerConfigResponse, len(items))
	for i, item := range items {
		responses[i] = ResponseFrom(item)
	}

	return responses, nil
}

// GetByID returns a single MCP server config by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (McpServerConfigResponse, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return McpServerConfigResponse{}, err
	}
	return ResponseFrom(c), nil
}

// Create creates a new MCP server config.
func (s *Service) Create(ctx context.Context, req CreateRequest) (McpServerConfigResponse, error) {
	if req.Name == "" {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: name is required")
	}
	if req.TransportType == "" {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: transport type is required")
	}
	if req.TransportType == "stdio" {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: stdio transport is no longer supported, use http")
	}

	c := McpServerConfig{
		Name:              req.Name,
		TransportType:     req.TransportType,
		HTTPBaseURL:       req.HTTPBaseURL,
		Command:           req.Command,
		Args:              req.Args,
		Env:               req.Env,
		OAuthCredentialID: req.OAuthCredentialID,
		AutoStart:         req.AutoStart,
		Enabled:           req.Enabled,
	}

	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: create: %w", err)
	}

	return ResponseFrom(created), nil
}

// Update updates an existing MCP server config.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (McpServerConfigResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return McpServerConfigResponse{}, err
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.TransportType != nil {
		if *req.TransportType == "stdio" {
			return McpServerConfigResponse{}, fmt.Errorf("mcp service: stdio transport is no longer supported, use http")
		}
		existing.TransportType = *req.TransportType
	}
	if req.HTTPBaseURL != nil {
		existing.HTTPBaseURL = req.HTTPBaseURL
	}
	if req.Command != nil {
		existing.Command = req.Command
	}
	if req.Args != nil {
		existing.Args = *req.Args
	}
	if req.Env != nil {
		existing.Env = *req.Env
	}
	if req.OAuthCredentialID != nil {
		existing.OAuthCredentialID = req.OAuthCredentialID
	}
	if req.AutoStart != nil {
		existing.AutoStart = *req.AutoStart
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: update: %w", err)
	}

	return ResponseFrom(updated), nil
}

// Delete removes an MCP server config by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("mcp service: delete: %w", err)
	}
	return nil
}

// ListAutoStart returns all auto-start enabled MCP server configs.
func (s *Service) ListAutoStart(ctx context.Context) ([]McpServerConfigResponse, error) {
	items, err := s.repo.ListAutoStart(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp service: list auto-start: %w", err)
	}

	responses := make([]McpServerConfigResponse, len(items))
	for i, item := range items {
		responses[i] = ResponseFrom(item)
	}

	return responses, nil
}

// GetAuthStatus returns the current authentication status for an MCP server.
func (s *Service) GetAuthStatus(ctx context.Context, id uuid.UUID) (AuthStatusResponse, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return AuthStatusResponse{}, err
	}

	res := AuthStatusResponse{Authenticated: false}

	// 1. Check if we have local discovery metadata from runtime
	if s.mcpRuntimeURL != "" {
		url := fmt.Sprintf("%s/servers/%s/status", s.mcpRuntimeURL, config.Name)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var runtimeStatus McpRuntimeStatus
			if err := json.NewDecoder(resp.Body).Decode(&runtimeStatus); err == nil {
				res.Metadata = runtimeStatus.AuthMetadata
			}
			resp.Body.Close()
		}
	}

	// 2. Check if we have a linked credential with a token
	if config.OAuthCredentialID != nil {
		// In a real implementation, we would check if the token is valid.
		// For now, if we have a linked credential, we consider it "potentially authenticated".
		res.Authenticated = true
	}

	return res, nil
}

// GetConnectURL returns the URL to start OAuth flow for the MCP server.
func (s *Service) GetConnectURL(ctx context.Context, id uuid.UUID, redirectURL string) (ConnectURLResponse, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == "" {
		return ConnectURLResponse{}, fmt.Errorf("mcp service: tenant context is required")
	}

	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ConnectURLResponse{}, err
	}

	// 1. Check if we have local discovery metadata from runtime
	discoveredAuthURL := ""
	discoveredTokenURL := ""
	discoveredScopes := ""

	if s.mcpRuntimeURL != "" {
		url := fmt.Sprintf("%s/servers/%s/status", s.mcpRuntimeURL, config.Name)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var runtimeStatus McpRuntimeStatus
			if err := json.NewDecoder(resp.Body).Decode(&runtimeStatus); err == nil && runtimeStatus.AuthMetadata != nil {
				discoveredAuthURL = runtimeStatus.AuthMetadata.AuthorizationURL
				discoveredTokenURL = runtimeStatus.AuthMetadata.TokenURL
				if len(runtimeStatus.AuthMetadata.ScopesSupported) > 0 {
					discoveredScopes = strings.Join(runtimeStatus.AuthMetadata.ScopesSupported, ",")
				}
			}
			resp.Body.Close()
		}
	}

	authURL := ""
	tokenURL := ""
	clientID := ""
	scopes := ""

	// 2. Priority 1: Use discovered metadata from runtime (MCP Spec Discovery)
	if discoveredAuthURL != "" {
		authURL = discoveredAuthURL
		tokenURL = discoveredTokenURL
		scopes = discoveredScopes
	}

	// 3. Priority 2: If we have a linked OAuthCredential, use its configuration (can override discovery)
	if config.OAuthCredentialID != nil {
		if s.oauthSvc == nil {
			return ConnectURLResponse{}, fmt.Errorf("mcp service: OAuth service is not configured")
		}
		cred, err := s.oauthSvc.GetByID(ctx, tenantID, *config.OAuthCredentialID)
		if err != nil {
			return ConnectURLResponse{}, fmt.Errorf("mcp service: could not fetch linked OAuth credential (ID: %s) for tenant %s: %w", config.OAuthCredentialID.String(), tenantID, err)
		}

		if cred.AuthURL != nil && *cred.AuthURL != "" {
			authURL = *cred.AuthURL
		}
		if cred.TokenURL != nil && *cred.TokenURL != "" {
			tokenURL = *cred.TokenURL
		}
		if cred.ClientID != nil && *cred.ClientID != "" {
			clientID = *cred.ClientID
		}
		if cred.Scopes != nil && *cred.Scopes != "" {
			scopes = *cred.Scopes
		}
	}

	// 2. Simple discovery based on URL for known providers (fallback)
	url := ""
	if config.HTTPBaseURL != nil {
		url = *config.HTTPBaseURL
	}

	if authURL == "" {
		if strings.Contains(url, "github.com") {
			authURL = "https://github.com/login/oauth/authorize"
			tokenURL = "https://github.com/login/oauth/access_token"
			if scopes == "" {
				scopes = "repo,read:user,user:email"
			}
		} else if strings.Contains(url, "asana.com") {
			authURL = "https://app.asana.com/-/oauth_authorize"
			tokenURL = "https://app.asana.com/-/oauth_token"
			if scopes == "" {
				scopes = "default"
			}
		}
	}

	// 3. MCP Authorization Server Discovery (Draft)
	// If authURL is still empty, we could potentially try to fetch /.well-known/oauth-authorization-server
	// For now, we will assume that the user has to provide it in the Credential or it must be a known provider.
	// But let's add a placeholder for future implementation of RFC 8414 discovery.

	if authURL == "" {
		return ConnectURLResponse{}, fmt.Errorf("mcp service: authURL is empty for MCP %s (URL: %s). Ensure linked OAuth credential has AuthURL.", id.String(), url)
	}

	if clientID == "" {
		return ConnectURLResponse{}, fmt.Errorf("mcp service: clientID is empty for MCP %s. Ensure linked OAuth credential has ClientID.", id.String())
	}

	// PKCE is REQUIRED by OAuth 2.1 (and thus the MCP draft)
	verifier, challenge := "", ""
	if s.oauthSvc != nil {
		verifier, challenge = s.oauthSvc.GeneratePKCE()
	}

	finalURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		authURL, clientID, redirectURL, scopes, config.ID.String())

	if challenge != "" {
		finalURL += fmt.Sprintf("&code_challenge=%s&code_challenge_method=S256", challenge)
	}

	// 4. Persist flow state (Simplified)
	if s.oauthSvc != nil && config.OAuthCredentialID != nil {
		cred, _ := s.oauthSvc.GetByID(ctx, tenantID, *config.OAuthCredentialID)
		updateReq := oauth.CreateRequest{
			Name:         cred.Name,
			AuthType:     cred.AuthType,
			TokenURL:     &tokenURL,
			ClientID:     cred.ClientID,
			ClientSecret: cred.ClientSecret,
			Scopes:       cred.Scopes,
			APIKeyHeader: cred.APIKeyHeader,
			APIKeyValue:  cred.APIKeyValue,
			BearerToken:  cred.BearerToken,
			Username:     cred.Username,
			Password:     cred.Password,
			AuthURL:      &authURL,
			RedirectURL:  cred.RedirectURL,
			CodeVerifier: &verifier,
		}
		// If cred already has TokenURL/AuthURL, don't overwrite with fallbacks unless they were empty
		if cred.TokenURL != nil && *cred.TokenURL != "" {
			updateReq.TokenURL = cred.TokenURL
		}
		if cred.AuthURL != nil && *cred.AuthURL != "" {
			updateReq.AuthURL = cred.AuthURL
		}

		_, _ = s.oauthSvc.Update(ctx, tenantID, *config.OAuthCredentialID, updateReq)
	}

	return ConnectURLResponse{URL: finalURL}, nil
}
