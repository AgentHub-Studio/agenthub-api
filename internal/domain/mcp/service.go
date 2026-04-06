package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
)

// oauthService defines methods needed from oauth domain.
type oauthService interface {
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (oauth.OAuthCredential, error)
}

// Service provides business logic for McpServerConfig operations.
type Service struct {
	repo     Repository
	oauthSvc oauthService
}

// NewService creates a new Service backed by the given Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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

	if config.OAuthCredentialID == nil {
		return AuthStatusResponse{Authenticated: false}, nil
	}

	// In a real implementation, we would check if the token is valid.
	// For now, if we have a linked credential, we consider it "potentially authenticated".
	return AuthStatusResponse{Authenticated: true}, nil
}

// GetConnectURL returns the URL to start OAuth flow for the MCP server.
func (s *Service) GetConnectURL(ctx context.Context, id uuid.UUID, redirectURL string) (ConnectURLResponse, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ConnectURLResponse{}, err
	}

	// Simple discovery based on URL for known providers
	authURL := ""
	scopes := ""

	url := ""
	if config.HTTPBaseURL != nil {
		url = *config.HTTPBaseURL
	}

	if strings.Contains(url, "github.com") {
		authURL = "https://github.com/login/oauth/authorize"
		scopes = "repo,read:user,user:email"
	} else if strings.Contains(url, "asana.com") {
		authURL = "https://app.asana.com/-/oauth_authorize"
		scopes = "default"
	}

	if authURL == "" {
		return ConnectURLResponse{}, fmt.Errorf("mcp service: could not auto-discover OAuth endpoints for URL: %s", url)
	}

	// In a real flow, we would create/get an OAuthCredential of type AUTHORIZATION_CODE
	// and return the authorization URL.
	// For this task, we return a mock URL or a real one if we had ClientID configured.
	
	// We'll use a placeholder client_id for now or look up from a global config.
	clientID := "placeholder_client_id" 
	
	finalURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		authURL, clientID, redirectURL, scopes, config.ID.String())

	return ConnectURLResponse{URL: finalURL}, nil
}
