package agentic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"
)

const mcpRuntimeTokenRefreshSkew = 30 * time.Second

// MCPRuntimeTokenProvider supplies a Keycloak workload token for one tenant.
// It deliberately has no access to the end-user bearer token.
type MCPRuntimeTokenProvider interface {
	Token(ctx context.Context, tenantID string) (string, error)
}

type cachedMCPRuntimeToken struct {
	value     string
	expiresAt time.Time
}

// KeycloakServiceTokenProvider obtains a client-credentials token in the
// tenant realm. Keycloak must map the runtime audience to this service client.
type KeycloakServiceTokenProvider struct {
	keycloakBaseURL string
	clientID        string
	credentials     workloadidentity.Resolver
	scopes          []string
	httpClient      *http.Client

	mu    sync.Mutex
	cache map[string]cachedMCPRuntimeToken
}

// NewKeycloakServiceTokenProvider creates a provider for the internal
// agenthub-api -> mcp-client-runtime workload identity.
func NewKeycloakServiceTokenProvider(keycloakBaseURL, clientID string, credentials workloadidentity.Resolver, scopes []string) (*KeycloakServiceTokenProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(keycloakBaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("KEYCLOAK_BASE_URL is required for MCP runtime authentication")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("KEYCLOAK_BASE_URL must be an absolute HTTP(S) URL")
	}
	if strings.TrimSpace(clientID) == "" {
		return nil, errors.New("MCP_RUNTIME_CLIENT_ID is required")
	}
	if credentials == nil {
		return nil, errors.New("tenant workload credential resolver is required")
	}
	return &KeycloakServiceTokenProvider{
		keycloakBaseURL: baseURL,
		clientID:        strings.TrimSpace(clientID),
		credentials:     credentials,
		scopes:          append([]string(nil), scopes...),
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		cache:           make(map[string]cachedMCPRuntimeToken),
	}, nil
}

// Token returns a cached token when possible and otherwise requests one from
// Keycloak. The tenant is used only to select its realm/token endpoint.
func (p *KeycloakServiceTokenProvider) Token(ctx context.Context, tenantID string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "", errors.New("tenant is required to request MCP runtime token")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cached, ok := p.cache[tenantID]; ok && cached.value != "" && time.Until(cached.expiresAt) > mcpRuntimeTokenRefreshSkew {
		return cached.value, nil
	}
	credential, err := p.credentials.Resolve(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant workload credential: %w", err)
	}
	if credential.ClientID != p.clientID {
		return "", fmt.Errorf("tenant workload credential client ID %q does not match configured client ID %q", credential.ClientID, p.clientID)
	}
	if credential.ClientSecret == "" {
		return "", errors.New("tenant workload credential secret is empty")
	}

	form := url.Values{"grant_type": {"client_credentials"}}
	if len(p.scopes) > 0 {
		form.Set("scope", strings.Join(p.scopes, " "))
	}
	tokenURL := p.keycloakBaseURL + "/realms/" + url.PathEscape(tenantID) + "/protocol/openid-connect/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create Keycloak token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(credential.ClientID, credential.ClientSecret)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Keycloak service token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request Keycloak service token: unexpected status %d", resp.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Keycloak service token: %w", err)
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 {
		return "", errors.New("Keycloak token response is incomplete")
	}
	p.cache[tenantID] = cachedMCPRuntimeToken{
		value:     result.AccessToken,
		expiresAt: time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}
	return result.AccessToken, nil
}
