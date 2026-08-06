package workloadidentity

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
)

const tokenRefreshSkew = 30 * time.Second

// TokenProvider obtains a tenant-local workload token.
type TokenProvider interface {
	Token(ctx context.Context, tenantID string) (string, error)
}

type cachedToken struct {
	value     string
	expiresAt time.Time
}

// KeycloakTokenProvider exchanges each tenant's stored service credential for
// a Keycloak client-credentials token. It never accepts a user bearer token.
type KeycloakTokenProvider struct {
	keycloakBaseURL string
	clientID        string
	credentials     Resolver
	scopes          []string
	httpClient      *http.Client

	mu    sync.Mutex
	cache map[string]cachedToken
}

func NewKeycloakTokenProvider(keycloakBaseURL, clientID string, credentials Resolver, scopes []string) (*KeycloakTokenProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(keycloakBaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("KEYCLOAK_BASE_URL is required for workload authentication")
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
	return &KeycloakTokenProvider{
		keycloakBaseURL: baseURL,
		clientID:        strings.TrimSpace(clientID),
		credentials:     credentials,
		scopes:          append([]string(nil), scopes...),
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		cache:           make(map[string]cachedToken),
	}, nil
}

func (p *KeycloakTokenProvider) Token(ctx context.Context, tenantID string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "", errors.New("tenant is required to request workload token")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cached, ok := p.cache[tenantID]; ok && cached.value != "" && time.Until(cached.expiresAt) > tokenRefreshSkew {
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
	p.cache[tenantID] = cachedToken{value: result.AccessToken, expiresAt: time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)}
	return result.AccessToken, nil
}
