package oauth

import (
	"context"
	"crypto/sha256"
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

// encryptSecret encrypts s if an encryption key is configured.
func (s *Service) encryptSecret(plaintext *string) (*string, error) {
	if plaintext == nil || *plaintext == "" {
		return plaintext, nil
	}
	enc, err := crypto.Encrypt(s.encryptionKey, *plaintext)
	if err != nil {
		return nil, err
	}
	return &enc, nil
}

// decryptSecret decrypts s if an encryption key is configured.
func (s *Service) decryptSecret(ciphertext *string) (*string, error) {
	if ciphertext == nil || *ciphertext == "" {
		return ciphertext, nil
	}
	dec, err := crypto.Decrypt(s.encryptionKey, *ciphertext)
	if err != nil {
		return nil, err
	}
	return &dec, nil
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
		AuthURL:      req.AuthURL,
		RedirectURL:  req.RedirectURL,
		CodeVerifier: req.CodeVerifier,
	}
	return s.repo.Create(ctx, tenantID, c)
}

// Update updates an existing OAuth credential.
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (OAuthCredential, error) {
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return OAuthCredential{}, err
	}

	clientSecret, err := s.resolveUpdatedSecret(req.ClientSecret, existing.ClientSecret)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt client_secret: %w", err)
	}
	apiKeyValue, err := s.resolveUpdatedSecret(req.APIKeyValue, existing.APIKeyValue)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt api_key_value: %w", err)
	}
	bearerToken, err := s.resolveUpdatedSecret(req.BearerToken, existing.BearerToken)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: encrypt bearer_token: %w", err)
	}
	password, err := s.resolveUpdatedSecret(req.Password, existing.Password)
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
		AuthURL:      req.AuthURL,
		RedirectURL:  req.RedirectURL,
		CodeVerifier: req.CodeVerifier,
	}
	result, err := s.repo.Update(ctx, tenantID, id, c)
	if err == nil {
		s.mu.Lock()
		delete(s.tokenCache, id)
		s.mu.Unlock()
	}
	return result, err
}

func (s *Service) resolveUpdatedSecret(incoming *string, existing *string) (*string, error) {
	if incoming == nil || *incoming == "" || *incoming == "***" {
		return existing, nil
	}
	return s.encryptSecret(incoming)
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
func (s *Service) ResolveAuthHeader(ctx context.Context, tenantID string, id uuid.UUID) (ResolveResponse, error) {
	c, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return ResolveResponse{}, err
	}

	switch c.AuthType {
	case AuthTypeOAuth2ClientCredentials, AuthTypeOAuth2AuthorizationCode:
		// Check database status first
 	if c.AuthType == AuthTypeOAuth2AuthorizationCode {
			if c.BearerToken != nil && *c.BearerToken != "" && (c.ExpiresAt == nil || time.Now().Before(c.ExpiresAt.Add(-30*time.Second))) {
				tokenPtr, _ := s.decryptSecret(c.BearerToken)
				if tokenPtr != nil && *tokenPtr != "" {
					return ResolveResponse{Header: "Authorization", Value: "Bearer " + *tokenPtr}, nil
				}
			}
			// Needs refresh or exchange
			if c.RefreshToken != nil && *c.RefreshToken != "" {
				updated, err := s.RefreshToken(ctx, tenantID, id)
				if err == nil {
					tokenPtr, _ := s.decryptSecret(updated.BearerToken)
					if tokenPtr != nil && *tokenPtr != "" {
						return ResolveResponse{Header: "Authorization", Value: "Bearer " + *tokenPtr}, nil
					}
				}
			}
		}

		clientSecretPtr, _ := s.decryptSecret(c.ClientSecret)
		c.ClientSecret = clientSecretPtr
		token, err := s.fetchOrCachedToken(ctx, id, c)
		if err != nil {
			return ResolveResponse{}, err
		}
		return ResolveResponse{Header: "Authorization", Value: "Bearer " + token}, nil

	case AuthTypeBearerToken:
		bearerTokenPtr, _ := s.decryptSecret(c.BearerToken)
		val := ""
		if bearerTokenPtr != nil {
			val = *bearerTokenPtr
		}
		return ResolveResponse{Header: "Authorization", Value: "Bearer " + val}, nil

	case AuthTypeAPIKey:
		apiKeyValuePtr, _ := s.decryptSecret(c.APIKeyValue)
		header := ""
		if c.APIKeyHeader != nil {
			header = *c.APIKeyHeader
		}
		if header == "" {
			header = "X-API-Key"
		}
		val := ""
		if apiKeyValuePtr != nil {
			val = *apiKeyValuePtr
		}
		return ResolveResponse{Header: header, Value: val}, nil

	case AuthTypeBasicAuth:
		passwordPtr, _ := s.decryptSecret(c.Password)
		pass := ""
		if passwordPtr != nil {
			pass = *passwordPtr
		}
		user := ""
		if c.Username != nil {
			user = *c.Username
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		return ResolveResponse{Header: "Authorization", Value: "Basic " + encoded}, nil

	default:
		return ResolveResponse{}, fmt.Errorf("oauth: unsupported auth type: %s", c.AuthType)
	}
}

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

func (s *Service) exchangeClientCredentials(ctx context.Context, c OAuthCredential) (string, int, error) {
	params := url.Values{}
	params.Set("grant_type", "client_credentials")
	if c.ClientID != nil {
		params.Set("client_id", *c.ClientID)
	}
	if c.ClientSecret != nil {
		params.Set("client_secret", *c.ClientSecret)
	}
	if c.Scopes != nil && *c.Scopes != "" {
		params.Set("scope", *c.Scopes)
	}

	tokenURL := ""
	if c.TokenURL != nil {
		tokenURL = *c.TokenURL
	}
	return s.doTokenRequest(ctx, tokenURL, params)
}

func (s *Service) ExchangeCode(ctx context.Context, tenantID string, id uuid.UUID, code string) error {
	c, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}

	clientSecretPtr, _ := s.decryptSecret(c.ClientSecret)
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", code)
	if c.ClientID != nil {
		params.Set("client_id", *c.ClientID)
	}
	if clientSecretPtr != nil {
		params.Set("client_secret", *clientSecretPtr)
	}
	if c.RedirectURL != nil {
		params.Set("redirect_uri", *c.RedirectURL)
	}
	if c.CodeVerifier != nil && *c.CodeVerifier != "" {
		params.Set("code_verifier", *c.CodeVerifier)
	}

	tokenURL := ""
	if c.TokenURL != nil {
		tokenURL = *c.TokenURL
	}
	token, refreshToken, expiresIn, err := s.doFullTokenRequest(ctx, tokenURL, params)
	if err != nil {
		return err
	}

	encToken, _ := s.encryptSecret(&token)
	encRefresh, _ := s.encryptSecret(&refreshToken)
	expiry := time.Now().Add(time.Duration(expiresIn) * time.Second)

	c.BearerToken = encToken
	c.RefreshToken = encRefresh
	c.ExpiresAt = &expiry

	_, err = s.repo.Update(ctx, tenantID, id, c)
	return err
}

func (s *Service) RefreshToken(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error) {
	c, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return OAuthCredential{}, err
	}

	clientSecretPtr, _ := s.decryptSecret(c.ClientSecret)
	refreshTokenPtr, _ := s.decryptSecret(c.RefreshToken)

	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	if refreshTokenPtr != nil {
		params.Set("refresh_token", *refreshTokenPtr)
	}
	if c.ClientID != nil {
		params.Set("client_id", *c.ClientID)
	}
	if clientSecretPtr != nil {
		params.Set("client_secret", *clientSecretPtr)
	}

	tokenURL := ""
	if c.TokenURL != nil {
		tokenURL = *c.TokenURL
	}
	token, newRefresh, expiresIn, err := s.doFullTokenRequest(ctx, tokenURL, params)
	if err != nil {
		return OAuthCredential{}, err
	}

	encToken, _ := s.encryptSecret(&token)
	expiry := time.Now().Add(time.Duration(expiresIn) * time.Second)

	c.BearerToken = encToken
	if newRefresh != "" {
		encRefresh, _ := s.encryptSecret(&newRefresh)
		c.RefreshToken = encRefresh
	}
	c.ExpiresAt = &expiry

	return s.repo.Update(ctx, tenantID, id, c)
}

func (s *Service) doTokenRequest(ctx context.Context, tokenURL string, params url.Values) (string, int, error) {
	token, _, expiresIn, err := s.doFullTokenRequest(ctx, tokenURL, params)
	return token, expiresIn, err
}

func (s *Service) doFullTokenRequest(ctx context.Context, tokenURL string, params url.Values) (string, string, int, error) {
	if tokenURL == "" {
		return "", "", 0, fmt.Errorf("oauth: token_url is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("oauth: token endpoint returned %d", resp.StatusCode)
	}

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", 0, err
	}
	if body.ExpiresIn <= 0 {
		body.ExpiresIn = 3600
	}
	return body.AccessToken, body.RefreshToken, body.ExpiresIn, nil
}

// GeneratePKCE creates a code_verifier and code_challenge (S256).
func (s *Service) GeneratePKCE() (verifier string, challenge string) {
	verifier = base64.RawURLEncoding.EncodeToString(crypto.GenerateRandomKey(32))
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge
}
