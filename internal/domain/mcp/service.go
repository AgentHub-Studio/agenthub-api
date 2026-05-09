package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// oauthService defines methods needed from oauth domain.
type oauthService interface {
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (oauth.OAuthCredential, error)
	Create(ctx context.Context, tenantID string, req oauth.CreateRequest) (oauth.OAuthCredential, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req oauth.CreateRequest) (oauth.OAuthCredential, error)
	ExchangeCode(ctx context.Context, tenantID string, id uuid.UUID, code string) error
	DecryptSecret(ciphertext *string) (*string, error)
	GeneratePKCE() (string, string)
}

// Service provides business logic for McpServerConfig operations.
type Service struct {
	repo     Repository
	oauthSvc oauthService
	// Discovery from runtime
	mcpRuntimeURL string
}

// AuthServerMetadata represents OAuth 2.0 Authorization Server Metadata (RFC 8414).
// Mirrors the structure used by mcp-go for spec compliance.
type AuthServerMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint,omitempty"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
}

// OAuthProtectedResource represents the response from /.well-known/oauth-protected-resource (RFC 9728).
type OAuthProtectedResource struct {
	AuthorizationServers []string `json:"authorization_servers"`
	Resource             string   `json:"resource"`
}

type DCRResponse struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
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
	if req.TransportType != "http" {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: transport type must be 'http' (got %q)", req.TransportType)
	}
	if req.HTTPBaseURL == nil || strings.TrimSpace(*req.HTTPBaseURL) == "" {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: httpBaseUrl is required for http transport")
	}
	if err := ssrf.ValidateURL(*req.HTTPBaseURL); err != nil {
		return McpServerConfigResponse{}, fmt.Errorf("mcp service: httpBaseUrl invalid (%v)", err)
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
		// Detecta unique constraint violation (SQLSTATE 23505) e
		// retorna sentinel limpo, permitindo handler mapear para 409.
		msg := err.Error()
		for i := 0; i+5 <= len(msg); i++ {
			if msg[i:i+5] == "23505" {
				return McpServerConfigResponse{}, ErrDuplicateName
			}
		}
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
		// Bug 115: name vazio causa server config sem label na UI.
		// Create rejeita; Update precisa do mesmo gate.
		if *req.Name == "" {
			return McpServerConfigResponse{}, fmt.Errorf("mcp service: name cannot be empty")
		}
		existing.Name = *req.Name
	}
	if req.TransportType != nil {
		// Bug 119a: aceita só "http" (stdio foi removido). Sem este
		// gate, admin podia salvar transportType="INVALID" e o
		// runtime do MCP client falharia ao montar o transporte.
		if *req.TransportType != "http" {
			if *req.TransportType == "stdio" {
				return McpServerConfigResponse{}, fmt.Errorf("mcp service: stdio transport is no longer supported, use http")
			}
			return McpServerConfigResponse{}, fmt.Errorf("mcp service: transport type must be 'http' (got %q)", *req.TransportType)
		}
		existing.TransportType = *req.TransportType
	}
	if req.HTTPBaseURL != nil {
		// Bug 119b: httpBaseUrl="" deixa config inválido para http
		// transport. Se admin quer manter o valor anterior, deve omitir
		// o campo (PATCH true-partial) — não enviar string vazia.
		if strings.TrimSpace(*req.HTTPBaseURL) == "" {
			return McpServerConfigResponse{}, fmt.Errorf("mcp service: httpBaseUrl cannot be empty for http transport")
		}
		// Bug 104: Update precisa do mesmo gate SSRF que Create —
		// senão admin malicioso podia criar config benigno e depois
		// PATCH para http://localhost:9000/mcp.
		if err := ssrf.ValidateURL(*req.HTTPBaseURL); err != nil {
			return McpServerConfigResponse{}, fmt.Errorf("mcp service: httpBaseUrl invalid (%v)", err)
		}
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

// ListAllEnabled returns all enabled MCP server configs (regardless of auto_start).
// BUG-MCP-RUNTIME-STALE fix: used by the bootstrap endpoint so the mcp-client-runtime
// registers every enabled server, not only auto_start=true ones. This ensures servers
// created after startup are available for lazy connection on the first tool-list request.
func (s *Service) ListAllEnabled(ctx context.Context) ([]McpServerConfigResponse, error) {
	items, err := s.repo.ListAllEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp service: list all enabled: %w", err)
	}

	responses := make([]McpServerConfigResponse, len(items))
	for i, item := range items {
		responses[i] = ResponseFrom(item)
	}

	return responses, nil
}

// ListBootstrap returns enabled MCP configs with runtime-only OAuth material.
func (s *Service) ListBootstrap(ctx context.Context) ([]McpServerConfigBootstrapResponse, error) {
	items, err := s.repo.ListAllEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp service: list bootstrap: %w", err)
	}

	responses := make([]McpServerConfigBootstrapResponse, len(items))
	for i, item := range items {
		resp := BootstrapResponseFrom(item)
		if item.OAuthCredentialID != nil {
			if err := s.attachOAuthBootstrap(ctx, &resp, *item.OAuthCredentialID); err != nil {
				return nil, err
			}
		}
		responses[i] = resp
	}

	return responses, nil
}

func (s *Service) attachOAuthBootstrap(ctx context.Context, resp *McpServerConfigBootstrapResponse, credentialID uuid.UUID) error {
	if s.oauthSvc == nil {
		return nil
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == "" {
		return fmt.Errorf("mcp service: tenant context is required for OAuth bootstrap")
	}

	cred, err := s.oauthSvc.GetByID(ctx, tenantID, credentialID)
	if err != nil {
		return fmt.Errorf("mcp service: load oauth credential %s: %w", credentialID, err)
	}

	resp.OAuthTokenURL = stringValue(cred.TokenURL)
	resp.OAuthClientID = stringValue(cred.ClientID)
	resp.OAuthClientSecret, err = decryptedString(s.oauthSvc, cred.ClientSecret)
	if err != nil {
		return fmt.Errorf("mcp service: decrypt oauth client secret: %w", err)
	}
	resp.OAuthBearerToken, err = decryptedString(s.oauthSvc, cred.BearerToken)
	if err != nil {
		return fmt.Errorf("mcp service: decrypt oauth bearer token: %w", err)
	}
	resp.OAuthRefreshToken, err = decryptedString(s.oauthSvc, cred.RefreshToken)
	if err != nil {
		return fmt.Errorf("mcp service: decrypt oauth refresh token: %w", err)
	}
	resp.OAuthScopes = splitOAuthScopes(cred.Scopes)

	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func decryptedString(oauthSvc oauthService, encrypted *string) (string, error) {
	value, err := oauthSvc.DecryptSecret(encrypted)
	if err != nil {
		return "", err
	}
	return stringValue(value), nil
}

func splitOAuthScopes(scopes *string) []string {
	if scopes == nil || *scopes == "" {
		return nil
	}
	return strings.Fields(strings.ReplaceAll(*scopes, ",", " "))
}

// GetAuthStatus returns the current authentication status for an MCP server.
func (s *Service) GetAuthStatus(ctx context.Context, id uuid.UUID) (AuthStatusResponse, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return AuthStatusResponse{}, err
	}

	res := AuthStatusResponse{Authenticated: false}

	if config.OAuthCredentialID != nil {
		res.Authenticated = true
	}

	return res, nil
}

// GetConnectURL performs the full MCP OAuth 2.1 flow inline (following mcp-go's approach):
// 1. Discover auth server via RFC 9728 (/.well-known/oauth-protected-resource)
// 2. Fetch auth server metadata (RFC 8414 or OIDC Discovery)
// 3. Dynamic Client Registration (RFC 7591) if no clientID
// 4. Build authorization URL with PKCE
func (s *Service) GetConnectURL(ctx context.Context, id uuid.UUID, redirectURL string) (ConnectURLResponse, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == "" {
		return ConnectURLResponse{}, fmt.Errorf("mcp service: tenant context is required")
	}

	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ConnectURLResponse{}, err
	}

	mcpURL := ""
	if config.HTTPBaseURL != nil {
		mcpURL = *config.HTTPBaseURL
	}
	if mcpURL == "" {
		return ConnectURLResponse{}, fmt.Errorf("mcp service: HTTP base URL is required for MCP %s", id)
	}

	// --- Step 1: Discover auth server metadata (following mcp-go oauth.go logic) ---
	metadata, err := s.discoverAuthServerMetadata(ctx, mcpURL)
	if err != nil {
		return ConnectURLResponse{}, fmt.Errorf("mcp service: OAuth discovery failed for %s: %w", mcpURL, err)
	}

	log.Printf("mcp service: discovered metadata for %s: auth=%s token=%s reg=%s",
		mcpURL, metadata.AuthorizationEndpoint, metadata.TokenEndpoint, metadata.RegistrationEndpoint)

	// --- Step 2: Resolve clientID ---
	clientID := ""
	scopes := ""

	// Check linked credential first
	if config.OAuthCredentialID != nil && s.oauthSvc != nil {
		cred, credErr := s.oauthSvc.GetByID(ctx, tenantID, *config.OAuthCredentialID)
		if credErr == nil {
			if cred.ClientID != nil && *cred.ClientID != "" {
				clientID = *cred.ClientID
			}
			if cred.Scopes != nil && *cred.Scopes != "" {
				scopes = *cred.Scopes
			}
		}
	}

	// Use scopes from metadata if not set
	if scopes == "" && len(metadata.ScopesSupported) > 0 {
		scopes = strings.Join(metadata.ScopesSupported, " ")
	}

	// --- Step 3: Dynamic Client Registration (RFC 7591) if no clientID ---
	if clientID == "" && metadata.RegistrationEndpoint != "" {
		log.Printf("mcp service: performing DCR at %s for %s", metadata.RegistrationEndpoint, config.Name)
		dcrResp, dcrErr := s.performDCR(ctx, metadata.RegistrationEndpoint, redirectURL, scopes)
		if dcrErr != nil {
			log.Printf("mcp service: DCR failed for %s: %v", config.Name, dcrErr)
		} else {
			clientID = dcrResp.ClientID
			log.Printf("mcp service: DCR successful for %s, clientID=%s", config.Name, clientID)

			// Persist the DCR result as an OAuthCredential and link to MCP server
			if s.oauthSvc != nil {
				authURLStr := metadata.AuthorizationEndpoint
				tokenURLStr := metadata.TokenEndpoint
				newCred, createErr := s.oauthSvc.Create(ctx, tenantID, oauth.CreateRequest{
					Name:     config.Name + " OAuth (DCR)",
					AuthType: "oauth2",
					AuthURL:  &authURLStr,
					TokenURL: &tokenURLStr,
					ClientID: &clientID,
					Scopes:   &scopes,
				})
				if createErr == nil {
					config.OAuthCredentialID = &newCred.ID
					_, _ = s.repo.Update(ctx, config)
					log.Printf("mcp service: persisted DCR credential %s and linked to MCP %s", newCred.ID, config.ID)
				} else {
					log.Printf("mcp service: failed to persist DCR credential for %s: %v", config.Name, createErr)
				}
			}
		}
	}

	if clientID == "" {
		return ConnectURLResponse{}, fmt.Errorf(
			"mcp service: could not obtain clientID for MCP %s (URL: %s). "+
				"OAuth discovery succeeded (auth=%s) but Dynamic Client Registration failed or is not supported. "+
				"You may need to manually register an OAuth app and link the credential.",
			id, mcpURL, metadata.AuthorizationEndpoint)
	}

	// --- Step 4: Build authorization URL with PKCE ---
	verifier, challenge := "", ""
	if s.oauthSvc != nil {
		verifier, challenge = s.oauthSvc.GeneratePKCE()
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("state", config.ID.String())

	// Store the redirect_uri used so the callback can send the same value during token exchange
	redirectURLCopy := redirectURL
	if scopes != "" {
		params.Set("scope", scopes)
	}
	if challenge != "" {
		params.Set("code_challenge", challenge)
		params.Set("code_challenge_method", "S256")
	}

	finalURL := metadata.AuthorizationEndpoint + "?" + params.Encode()

	// Persist PKCE verifier and redirect_uri for the callback token exchange
	if s.oauthSvc != nil && config.OAuthCredentialID != nil {
		cred, credErr := s.oauthSvc.GetByID(ctx, tenantID, *config.OAuthCredentialID)
		if credErr == nil {
			authURLStr := metadata.AuthorizationEndpoint
			tokenURLStr := metadata.TokenEndpoint
			updateReq := oauth.CreateRequest{
				Name:         cred.Name,
				AuthType:     cred.AuthType,
				AuthURL:      &authURLStr,
				TokenURL:     &tokenURLStr,
				ClientID:     cred.ClientID,
				ClientSecret: cred.ClientSecret,
				Scopes:       cred.Scopes,
				APIKeyHeader: cred.APIKeyHeader,
				APIKeyValue:  cred.APIKeyValue,
				BearerToken:  cred.BearerToken,
				Username:     cred.Username,
				Password:     cred.Password,
				RedirectURL:  &redirectURLCopy,
				CodeVerifier: &verifier,
			}
			_, _ = s.oauthSvc.Update(ctx, tenantID, *config.OAuthCredentialID, updateReq)
		}
	}

	return ConnectURLResponse{URL: finalURL}, nil
}

// HandleOAuthCallback processes the authorization code returned by the OAuth provider.
// The MCP server ID is used as the OAuth state parameter. This method looks up the
// linked OAuthCredential and delegates the code-for-token exchange to the oauth service.
func (s *Service) HandleOAuthCallback(ctx context.Context, mcpServerID uuid.UUID, code string) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID == "" {
		return fmt.Errorf("mcp service: tenant context is required")
	}

	config, err := s.repo.GetByID(ctx, mcpServerID)
	if err != nil {
		return fmt.Errorf("mcp service: callback: %w", err)
	}

	if config.OAuthCredentialID == nil {
		return fmt.Errorf("mcp service: callback: MCP server %s has no linked OAuth credential", mcpServerID)
	}

	if s.oauthSvc == nil {
		return fmt.Errorf("mcp service: callback: oauth service not available")
	}

	log.Printf("mcp service: exchanging authorization code for MCP %s (credential %s)", mcpServerID, *config.OAuthCredentialID)

	if err := s.oauthSvc.ExchangeCode(ctx, tenantID, *config.OAuthCredentialID, code); err != nil {
		return fmt.Errorf("mcp service: callback: token exchange failed: %w", err)
	}

	// Unregister the server from the runtime so that the next request forces a reload of the new tokens
	if s.mcpRuntimeURL != "" {
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/servers/%s", s.mcpRuntimeURL, config.Name), nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			log.Printf("mcp service: unregistered server %s from runtime to force token reload", config.Name)
		} else {
			log.Printf("mcp service: failed to unregister server %s: %v", config.Name, err)
		}
	}

	log.Printf("mcp service: OAuth token exchange successful for MCP %s", mcpServerID)
	return nil
}

// ListTools fetches the tools exposed by an MCP server from the runtime.
func (s *Service) ListTools(ctx context.Context, id uuid.UUID) ([]ToolResponse, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("mcp service: tools: %w", err)
	}

	if s.mcpRuntimeURL == "" {
		return nil, fmt.Errorf("mcp service: tools: MCP runtime URL not configured")
	}

	url := fmt.Sprintf("%s/servers/%s/tools", s.mcpRuntimeURL, config.Name)
	log.Printf("mcp service: fetching tools from %s", url)

	// Ensure server is registered in runtime before fetching tools
	if err := s.ensureServerRegistered(ctx, config); err != nil {
		log.Printf("mcp service: failed to ensure server registration for %s: %v", config.Name, err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("mcp service: tools: failed to contact runtime: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		var errResp struct {
			Error        string `json:"error"`
			AuthRequired bool   `json:"auth_required"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("mcp service: tools: OAuth token expired or invalid for server '%s'. Please reconnect via the MCP server list (click 'Connect')", config.Name)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("mcp service: tools: runtime error: %s", errResp.Error)
	}

	var result struct {
		Tools []ToolResponse `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("mcp service: tools: failed to decode response: %w", err)
	}

	return result.Tools, nil
}

// ensureServerRegistered checks if an MCP server is registered in the runtime and registers it if missing.
func (s *Service) ensureServerRegistered(ctx context.Context, config McpServerConfig) error {
	if s.mcpRuntimeURL == "" {
		return fmt.Errorf("MCP runtime URL not configured")
	}

	// 1. Check if registered
	statusURL := fmt.Sprintf("%s/servers/%s/status", s.mcpRuntimeURL, config.Name)
	for i := 0; i < 5; i++ {
		resp, err := http.Get(statusURL)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				// Already registered, check status
				var status struct {
					Status string `json:"Status"`
				}
				if decodeErr := json.NewDecoder(resp.Body).Decode(&status); decodeErr == nil {
					if status.Status == "running" {
						return nil
					}
					log.Printf("mcp service: server %s registered but status is %s, starting...", config.Name, status.Status)

					// Try to start explicitly
					startURL := fmt.Sprintf("%s/servers/%s/start", s.mcpRuntimeURL, config.Name)
					startResp, startErr := http.Post(startURL, "application/json", nil)
					if startErr == nil {
						defer startResp.Body.Close()
						if startResp.StatusCode == http.StatusUnauthorized {
							return fmt.Errorf("OAuth token expired or invalid for server '%s'. Please reconnect via the MCP server list (click 'Connect')", config.Name)
						}
					}
				}
			}
		}
		time.Sleep(2 * time.Second)
	}

	// 2. Not registered or not running, register/start it
	mcpURL := ""
	if config.HTTPBaseURL != nil {
		mcpURL = *config.HTTPBaseURL
	}

	regReq := map[string]interface{}{
		"name":          config.Name,
		"transportType": config.TransportType,
		"httpBaseUrl":   mcpURL,
		"autoStart":     true,
		"enabled":       true,
	}

	// Add OAuth credentials if available
	tenantID := tenant.FromContext(ctx)
	if tenantID != "" && config.OAuthCredentialID != nil && s.oauthSvc != nil {
		cred, credErr := s.oauthSvc.GetByID(ctx, tenantID, *config.OAuthCredentialID)
		if credErr == nil {
			if cred.TokenURL != nil {
				regReq["oauthTokenUrl"] = *cred.TokenURL
			}
			if cred.ClientID != nil {
				regReq["oauthClientId"] = *cred.ClientID
			}
			if cred.ClientSecret != nil {
				regReq["oauthClientSecret"] = *cred.ClientSecret
			}
			if cred.Scopes != nil {
				regReq["oauthScopes"] = strings.Split(*cred.Scopes, " ")
			}
			if cred.BearerToken != nil {
				if dec, err := s.oauthSvc.DecryptSecret(cred.BearerToken); err == nil && dec != nil {
					regReq["oauthBearerToken"] = *dec
				}
			}
			if cred.RefreshToken != nil {
				if dec, err := s.oauthSvc.DecryptSecret(cred.RefreshToken); err == nil && dec != nil {
					regReq["oauthRefreshToken"] = *dec
				}
			}
		}
	}

	body, _ := json.Marshal(regReq)
	regURL := fmt.Sprintf("%s/servers", s.mcpRuntimeURL)
	resp, err := http.Post(regURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to register server in runtime: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("runtime registration failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Give it a moment to initialize
	time.Sleep(1 * time.Second)
	return nil
}

// discoverAuthServerMetadata implements the MCP spec discovery flow (mirrors mcp-go oauth.go getServerMetadata):
// 1. Try /.well-known/oauth-protected-resource on the MCP server (RFC 9728)
// 2. If that returns authorization_servers, fetch metadata from the first one
// 3. Fallback to /.well-known/oauth-authorization-server on the MCP server
// 4. Fallback to /.well-known/openid-configuration on the auth server
// 5. Last resort: default endpoints based on the MCP server URL
func (s *Service) discoverAuthServerMetadata(ctx context.Context, mcpURL string) (*AuthServerMetadata, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}

	// Step 1: Try RFC 9728 Protected Resource Metadata
	prURL, err := buildWellKnownURL(mcpURL, "oauth-protected-resource")
	if err == nil {
		log.Printf("mcp service: trying protected resource discovery at %s", prURL)
		pr, prErr := fetchJSON[OAuthProtectedResource](ctx, httpClient, prURL)
		if prErr == nil && len(pr.AuthorizationServers) > 0 {
			authServerURL := pr.AuthorizationServers[0]
			log.Printf("mcp service: found auth server %s via protected resource metadata", authServerURL)

			// Fetch metadata from the discovered auth server
			meta := s.fetchAuthServerMetadata(ctx, httpClient, authServerURL)
			if meta != nil {
				return meta, nil
			}
		}
	}

	// Step 2: Fallback - try /.well-known/oauth-authorization-server on the MCP server itself
	asURL, err := buildWellKnownURL(mcpURL, "oauth-authorization-server")
	if err == nil {
		log.Printf("mcp service: trying oauth-authorization-server at %s", asURL)
		meta, metaErr := fetchJSON[AuthServerMetadata](ctx, httpClient, asURL)
		if metaErr == nil && meta.AuthorizationEndpoint != "" {
			return &meta, nil
		}
	}

	// Step 3: Extract host from MCP URL and try OIDC/RFC 8414 on the host root
	parsed, err := url.Parse(mcpURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP URL: %w", err)
	}
	hostRoot := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	meta := s.fetchAuthServerMetadata(ctx, httpClient, hostRoot)
	if meta != nil {
		return meta, nil
	}

	// Step 4: Default endpoints (last resort, same as mcp-go getDefaultEndpoints)
	log.Printf("mcp service: using default endpoints for %s", hostRoot)
	return &AuthServerMetadata{
		Issuer:                hostRoot,
		AuthorizationEndpoint: hostRoot + "/authorize",
		TokenEndpoint:         hostRoot + "/token",
		RegistrationEndpoint:  hostRoot + "/register",
	}, nil
}

// fetchAuthServerMetadata tries RFC 8414 and OIDC Discovery on a given auth server URL.
func (s *Service) fetchAuthServerMetadata(ctx context.Context, httpClient *http.Client, authServerURL string) *AuthServerMetadata {
	// Try RFC 8414 first
	asMetaURL, err := buildWellKnownURL(authServerURL, "oauth-authorization-server")
	if err == nil {
		meta, metaErr := fetchJSON[AuthServerMetadata](ctx, httpClient, asMetaURL)
		if metaErr == nil && meta.AuthorizationEndpoint != "" {
			log.Printf("mcp service: found auth metadata via RFC 8414 at %s", asMetaURL)
			return &meta
		}
	}

	// Try OIDC Discovery
	oidcURL, err := buildWellKnownURL(authServerURL, "openid-configuration")
	if err == nil {
		meta, metaErr := fetchJSON[AuthServerMetadata](ctx, httpClient, oidcURL)
		if metaErr == nil && meta.AuthorizationEndpoint != "" {
			log.Printf("mcp service: found auth metadata via OIDC at %s", oidcURL)
			return &meta
		}
	}

	return nil
}

// buildWellKnownURL constructs a well-known URL following the same logic as mcp-go:
// For a URL like https://example.com/v1/mcp, it produces:
// https://example.com/.well-known/{suffix}/v1/mcp
func buildWellKnownURL(baseURL string, suffix string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid URL: missing scheme or host in %q", baseURL)
	}

	path := strings.TrimSuffix(parsed.EscapedPath(), "/")
	root := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	if path == "" || path == "/" {
		return root + "/.well-known/" + suffix, nil
	}
	return root + "/.well-known/" + suffix + path, nil
}

// performDCR performs Dynamic Client Registration (RFC 7591).
func (s *Service) performDCR(ctx context.Context, registrationEndpoint, redirectURI, scopes string) (*DCRResponse, error) {
	regRequest := map[string]interface{}{
		"client_name":                "AgentHub MCP Client",
		"redirect_uris":              []string{redirectURI},
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	}
	if scopes != "" {
		regRequest["scope"] = scopes
	}

	reqBody, err := json.Marshal(regRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DCR request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create DCR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DCR request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DCR failed (status %d): %s", resp.StatusCode, string(body))
	}

	var dcrResp DCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcrResp); err != nil {
		return nil, fmt.Errorf("failed to decode DCR response: %w", err)
	}
	if dcrResp.ClientID == "" {
		return nil, fmt.Errorf("DCR response missing client_id")
	}

	return &dcrResp, nil
}

// fetchJSON is a generic helper to GET a URL and decode JSON.
func fetchJSON[T any](ctx context.Context, httpClient *http.Client, targetURL string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-03-26")

	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("HTTP %d from %s", resp.StatusCode, targetURL)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, err
	}
	return result, nil
}
