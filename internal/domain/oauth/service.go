package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/crypto"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// CredentialRepository defines the persistence interface for OAuthCredential.
type CredentialRepository interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]OAuthCredential, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error)
	Create(ctx context.Context, tenantID string, c OAuthCredential) (OAuthCredential, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, c OAuthCredential) (OAuthCredential, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
}

// cachedToken stores a fetched OAuth2 access token with its expiry.
type cachedToken struct {
	accessToken string
	expiresAt   time.Time
}

// isValid returns true when the cached token has not yet expired (with a 30s buffer).
func (t *cachedToken) isValid() bool {
	return time.Now().Before(t.expiresAt.Add(-30 * time.Second))
}

// HTTPClient is the interface used for OAuth2 token requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Service implements business logic for OAuth credentials.
type Service struct {
	repo          CredentialRepository
	httpClient    HTTPClient
	encryptionKey string // AES-256-GCM key; empty means no encryption (dev mode)

	mu         sync.Mutex
	tokenCache map[uuid.UUID]*cachedToken
}

// NewService creates a new Service without encryption.
func NewService(repo CredentialRepository) *Service {
	return &Service{
		repo:       repo,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		tokenCache: make(map[uuid.UUID]*cachedToken),
	}
}

// NewServiceWithEncryption creates a Service with AES-256-GCM secret encryption.
// key must be exactly 32 bytes; pass "" to disable encryption (development mode).
func NewServiceWithEncryption(repo CredentialRepository, key string) *Service {
	return &Service{
		repo:          repo,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		encryptionKey: key,
		tokenCache:    make(map[uuid.UUID]*cachedToken),
	}
}

// NewServiceWithClient creates a new Service with a custom HTTP client (useful for testing).
func NewServiceWithClient(repo CredentialRepository, client HTTPClient) *Service {
	return &Service{
		repo:       repo,
		httpClient: client,
		tokenCache: make(map[uuid.UUID]*cachedToken),
	}
}

// encryptSecret encrypts s if an encryption key is configured.
func (s *Service) encryptSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return crypto.Encrypt(s.encryptionKey, plaintext)
}

// decryptSecret decrypts s if an encryption key is configured.
func (s *Service) decryptSecret(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	return crypto.Decrypt(s.encryptionKey, ciphertext)
}

// ListAll returns a paginated list of OAuth credentials for the tenant.
func (s *Service) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]OAuthCredential, int, error) {
	return s.repo.ListAll(ctx, tenantID, pr)
}

// GetByID retrieves an OAuth credential by ID.
func (s *Service) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// Create creates a new OAuth credential, encrypting all secret fields.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (OAuthCredential, error) {
	clientSecret, err := s.encryptSecret(req.ClientSecret)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt client_secret: %w", err)
	}
	apiKeyValue, err := s.encryptSecret(req.APIKeyValue)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt api_key_value: %w", err)
	}
	bearerToken, err := s.encryptSecret(req.BearerToken)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt bearer_token: %w", err)
	}
	password, err := s.encryptSecret(req.Password)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt password: %w", err)
	}

	c := OAuthCredential{
		Name:         req.Name,
		AuthType:     req.AuthType,
		TokenURL:     req.TokenURL,
		ClientID:     req.ClientID,
		ClientSecret: clientSecret,
		Scopes:       req.Scopes,
		APIKeyHeader: req.APIKeyHeader,
		APIKeyValue:  apiKeyValue,
		BearerToken:  bearerToken,
		Username:     req.Username,
		Password:     password,
	}
	return s.repo.Create(ctx, tenantID, c)
}

// Update updates an existing OAuth credential, re-encrypting all secret fields, and clears its token cache entry.
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (OAuthCredential, error) {
	clientSecret, err := s.encryptSecret(req.ClientSecret)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt client_secret: %w", err)
	}
	apiKeyValue, err := s.encryptSecret(req.APIKeyValue)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt api_key_value: %w", err)
	}
	bearerToken, err := s.encryptSecret(req.BearerToken)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt bearer_token: %w", err)
	}
	password, err := s.encryptSecret(req.Password)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt password: %w", err)
	}

	c := OAuthCredential{
		Name:         req.Name,
		AuthType:     req.AuthType,
		TokenURL:     req.TokenURL,
		ClientID:     req.ClientID,
		ClientSecret: clientSecret,
		Scopes:       req.Scopes,
		APIKeyHeader: req.APIKeyHeader,
		APIKeyValue:  apiKeyValue,
		BearerToken:  bearerToken,
		Username:     req.Username,
		Password:     password,
	}
	result, err := s.repo.Update(ctx, tenantID, id, c)
	if err == nil {
		s.mu.Lock()
		delete(s.tokenCache, id)
		s.mu.Unlock()
	}
	return result, err
}

// Delete removes an OAuth credential and its token cache entry.
func (s *Service) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.tokenCache, id)
	s.mu.Unlock()
	return nil
}

// ResolveAuthHeader resolves the credential to an HTTP Authorization header value.
// For OAUTH2_CLIENT_CREDENTIALS the token is fetched (and cached) from the token URL.
func (s *Service) ResolveAuthHeader(ctx context.Context, tenantID string, id uuid.UUID) (ResolveResponse, error) {
	c, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return ResolveResponse{}, err
	}

	switch c.AuthType {
	case AuthTypeOAuth2ClientCredentials:
		// Decrypt client_secret before performing token exchange.
		clientSecret, err := s.decryptSecret(c.ClientSecret)
		if err != nil {
			return ResolveResponse{}, fmt.Errorf("oauth: decrypt client_secret: %w", err)
		}
		c.ClientSecret = clientSecret
		token, err := s.fetchOrCachedToken(ctx, id, c)
		if err != nil {
			return ResolveResponse{}, err
		}
		return ResolveResponse{Header: "Authorization", Value: "Bearer " + token}, nil

	case AuthTypeBearerToken:
		bearerToken, err := s.decryptSecret(c.BearerToken)
		if err != nil {
			return ResolveResponse{}, fmt.Errorf("oauth: decrypt bearer_token: %w", err)
		}
		return ResolveResponse{Header: "Authorization", Value: "Bearer " + bearerToken}, nil

	case AuthTypeAPIKey:
		apiKeyValue, err := s.decryptSecret(c.APIKeyValue)
		if err != nil {
			return ResolveResponse{}, fmt.Errorf("oauth: decrypt api_key_value: %w", err)
		}
		header := c.APIKeyHeader
		if header == "" {
			header = "X-API-Key"
		}
		return ResolveResponse{Header: header, Value: apiKeyValue}, nil

	case AuthTypeBasicAuth:
		password, err := s.decryptSecret(c.Password)
		if err != nil {
			return ResolveResponse{}, fmt.Errorf("oauth: decrypt password: %w", err)
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(c.Username + ":" + password))
		return ResolveResponse{Header: "Authorization", Value: "Basic " + encoded}, nil

	default:
		return ResolveResponse{}, fmt.Errorf("oauth: unsupported auth type: %s", c.AuthType)
	}
}

// fetchOrCachedToken returns a valid access token for the credential, using the
// in-memory cache when the cached token is still valid.
func (s *Service) fetchOrCachedToken(ctx context.Context, id uuid.UUID, c OAuthCredential) (string, error) {
	s.mu.Lock()
	if cached, ok := s.tokenCache[id]; ok && cached.isValid() {
		s.mu.Unlock()
		return cached.accessToken, nil
	}
	s.mu.Unlock()

	token, expiresIn, err := s.exchangeClientCredentials(ctx, c)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.tokenCache[id] = &cachedToken{
		accessToken: token,
		expiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	s.mu.Unlock()
	return token, nil
}

// exchangeClientCredentials performs an OAuth2 Client Credentials grant request.
func (s *Service) exchangeClientCredentials(ctx context.Context, c OAuthCredential) (token string, expiresIn int, err error) {
	if c.TokenURL == "" {
		return "", 0, fmt.Errorf("oauth: token_url is required for CLIENT_CREDENTIALS flow")
	}

	params := url.Values{}
	params.Set("grant_type", "client_credentials")
	params.Set("client_id", c.ClientID)
	params.Set("client_secret", c.ClientSecret)
	if c.Scopes != "" {
		params.Set("scope", c.Scopes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL,
		strings.NewReader(params.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("oauth: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("oauth: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("oauth: token endpoint returned %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", 0, fmt.Errorf("oauth: decode token response: %w", err)
	}
	if body.AccessToken == "" {
		return "", 0, fmt.Errorf("oauth: empty access_token in response")
	}
	if body.ExpiresIn <= 0 {
		body.ExpiresIn = 3600 // default 1h when not provided
	}
	return body.AccessToken, body.ExpiresIn, nil
}
