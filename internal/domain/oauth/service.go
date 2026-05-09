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
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

// CredentialRepository defines the persistence interface for OAuthCredential.
type CredentialRepository interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]OAuthCredential, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error)
	ExistsByName(ctx context.Context, tenantID, name string) (bool, error)
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

// NewServiceWithClient creates a Service that uses a custom HTTPClient for OAuth2
// token requests. Useful for testing without a real network.
func NewServiceWithClient(repo CredentialRepository, client HTTPClient) *Service {
	return &Service{
		repo:       repo,
		httpClient: client,
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
func (s *Service) DecryptSecret(ciphertext *string) (*string, error) {
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
//
// Validation precede encryption — without it, body {} produzia
// credenciais com name="" e authType="" salvas no banco (lixo
// inerte que populava listings de Integrações sem efeito útil).
// O handler converte ErrValidation em 422.
func validateCreateRequest(req CreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	// Bug 129: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	// Bug 164: cap URLs em 2048 chars (RFC standard).
	if !ptrEmpty(req.TokenURL) && len(*req.TokenURL) > 2048 {
		return fmt.Errorf("%w: tokenUrl exceeds maximum length of 2048 chars (got %d)", ErrValidation, len(*req.TokenURL))
	}
	if !ptrEmpty(req.AuthURL) && len(*req.AuthURL) > 2048 {
		return fmt.Errorf("%w: authUrl exceeds maximum length of 2048 chars (got %d)", ErrValidation, len(*req.AuthURL))
	}
	if !ptrEmpty(req.RedirectURL) && len(*req.RedirectURL) > 2048 {
		return fmt.Errorf("%w: redirectUrl exceeds maximum length of 2048 chars (got %d)", ErrValidation, len(*req.RedirectURL))
	}
	switch req.AuthType {
	case AuthTypeOAuth2ClientCredentials:
		if ptrEmpty(req.TokenURL) {
			return fmt.Errorf("%w: tokenUrl is required for OAUTH2_CLIENT_CREDENTIALS", ErrValidation)
		}
		if err := ssrf.ValidateURL(*req.TokenURL); err != nil {
			return fmt.Errorf("%w: tokenUrl invalid (%v)", ErrValidation, err)
		}
		if ptrEmpty(req.ClientID) {
			return fmt.Errorf("%w: clientId is required for OAUTH2_CLIENT_CREDENTIALS", ErrValidation)
		}
		if ptrEmpty(req.ClientSecret) {
			return fmt.Errorf("%w: clientSecret is required for OAUTH2_CLIENT_CREDENTIALS", ErrValidation)
		}
	case AuthTypeOAuth2AuthorizationCode:
		if ptrEmpty(req.AuthURL) {
			return fmt.Errorf("%w: authUrl is required for OAUTH2_AUTHORIZATION_CODE", ErrValidation)
		}
		if err := ssrf.ValidateURL(*req.AuthURL); err != nil {
			return fmt.Errorf("%w: authUrl invalid (%v)", ErrValidation, err)
		}
		if ptrEmpty(req.TokenURL) {
			return fmt.Errorf("%w: tokenUrl is required for OAUTH2_AUTHORIZATION_CODE", ErrValidation)
		}
		if err := ssrf.ValidateURL(*req.TokenURL); err != nil {
			return fmt.Errorf("%w: tokenUrl invalid (%v)", ErrValidation, err)
		}
		// Bug 105: redirectUrl deve passar pelo mesmo gate SSRF que
		// tokenUrl/authUrl. É opcional (nem todo flow OAuth2 usa
		// redirect_uri customizado), mas se vier, não pode apontar
		// para localhost / 169.254 / cluster DNS.
		if !ptrEmpty(req.RedirectURL) {
			if err := ssrf.ValidateURL(*req.RedirectURL); err != nil {
				return fmt.Errorf("%w: redirectUrl invalid (%v)", ErrValidation, err)
			}
		}
		if ptrEmpty(req.ClientID) {
			return fmt.Errorf("%w: clientId is required for OAUTH2_AUTHORIZATION_CODE", ErrValidation)
		}
	case AuthTypeAPIKey:
		if ptrEmpty(req.APIKeyValue) {
			return fmt.Errorf("%w: apiKeyValue is required for API_KEY", ErrValidation)
		}
	case AuthTypeBearerToken:
		if ptrEmpty(req.BearerToken) {
			return fmt.Errorf("%w: bearerToken is required for BEARER_TOKEN", ErrValidation)
		}
	case AuthTypeBasicAuth:
		if ptrEmpty(req.Username) {
			return fmt.Errorf("%w: username is required for BASIC_AUTH", ErrValidation)
		}
		if ptrEmpty(req.Password) {
			return fmt.Errorf("%w: password is required for BASIC_AUTH", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: authType must be one of OAUTH2_CLIENT_CREDENTIALS|OAUTH2_AUTHORIZATION_CODE|API_KEY|BEARER_TOKEN|BASIC_AUTH (got %q)",
			ErrValidation, req.AuthType)
	}
	return nil
}

func ptrEmpty(p *string) bool {
	return p == nil || strings.TrimSpace(*p) == ""
}

func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (OAuthCredential, error) {
	if err := validateCreateRequest(req); err != nil {
		return OAuthCredential{}, err
	}
	exists, err := s.repo.ExistsByName(ctx, tenantID, req.Name)
	if err != nil {
		return OAuthCredential{}, fmt.Errorf("oauth: check duplicate name: %w", err)
	}
	if exists {
		return OAuthCredential{}, ErrDuplicateName
	}
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
		Name:           req.Name,
		AuthType:       req.AuthType,
		TokenURL:       req.TokenURL,
		ClientID:       req.ClientID,
		ClientSecret:   clientSecret,
		Scopes:         req.Scopes,
		APIKeyHeader:   req.APIKeyHeader,
		APIKeyValue:    apiKeyValue,
		APIKeyLocation: normalizeAPIKeyLocation(req.APIKeyLocation),
		BearerToken:    bearerToken,
		Username:       req.Username,
		Password:       password,
		AuthURL:        req.AuthURL,
		RedirectURL:    req.RedirectURL,
		CodeVerifier:   req.CodeVerifier,
	}
	return s.repo.Create(ctx, tenantID, c)
}

// normalizeAPIKeyLocation defaults empty/unknown values to HEADER for backward compat.
func normalizeAPIKeyLocation(loc APIKeyLocation) APIKeyLocation {
	if loc == APIKeyLocationQuery {
		return APIKeyLocationQuery
	}
	return APIKeyLocationHeader
}

// Update updates an existing OAuth credential.
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (OAuthCredential, error) {
	// PATCH-friendly: get atual primeiro, preserva campos vazios
	// (true partial — backlog #186). Validação roda APÓS merge.
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return OAuthCredential{}, err
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = existing.Name
	}
	if req.AuthType == "" {
		req.AuthType = existing.AuthType
	}
	// Bug 139: per-authType fields (TokenURL, ClientID, etc.) também
	// precisam ser merged do existing para PATCH partial não fazer
	// validateCreateRequest falhar com "tokenUrl is required" quando
	// o usuário só está mudando name/description. Pointers nil no req
	// herdam do existing.
	if req.TokenURL == nil {
		req.TokenURL = existing.TokenURL
	}
	if req.ClientID == nil {
		req.ClientID = existing.ClientID
	}
	if req.Scopes == nil {
		req.Scopes = existing.Scopes
	}
	if req.APIKeyHeader == nil {
		req.APIKeyHeader = existing.APIKeyHeader
	}
	if req.APIKeyLocation == "" {
		req.APIKeyLocation = existing.APIKeyLocation
	}
	if req.AuthURL == nil {
		req.AuthURL = existing.AuthURL
	}
	if req.RedirectURL == nil {
		req.RedirectURL = existing.RedirectURL
	}
	if req.CodeVerifier == nil {
		req.CodeVerifier = existing.CodeVerifier
	}
	if req.Username == nil {
		req.Username = existing.Username
	}
	// Bug 139: secrets também precisam ser merged para passar pela
	// validação per-authType ("clientSecret is required for ..."),
	// mesmo quando o usuário não está rotacionando o segredo. O
	// resolveUpdatedSecret depois trata essa rota corretamente.
	if req.ClientSecret == nil {
		req.ClientSecret = existing.ClientSecret
	}
	if req.APIKeyValue == nil {
		req.APIKeyValue = existing.APIKeyValue
	}
	if req.BearerToken == nil {
		req.BearerToken = existing.BearerToken
	}
	if req.Password == nil {
		req.Password = existing.Password
	}
	// Validação após merge: name + authType + per-authType requirements.
	if err := validateCreateRequest(req); err != nil {
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
		Name:           req.Name,
		AuthType:       req.AuthType,
		TokenURL:       req.TokenURL,
		ClientID:       req.ClientID,
		ClientSecret:   clientSecret,
		Scopes:         req.Scopes,
		APIKeyHeader:   req.APIKeyHeader,
		APIKeyValue:    apiKeyValue,
		APIKeyLocation: normalizeAPIKeyLocation(req.APIKeyLocation),
		BearerToken:    bearerToken,
		Username:       req.Username,
		Password:       password,
		AuthURL:        req.AuthURL,
		RedirectURL:    req.RedirectURL,
		CodeVerifier:   req.CodeVerifier,
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
				tokenPtr, _ := s.DecryptSecret(c.BearerToken)
				if tokenPtr != nil && *tokenPtr != "" {
					return ResolveResponse{Header: "Authorization", Value: "Bearer " + *tokenPtr}, nil
				}
			}
			// Needs refresh or exchange
			if c.RefreshToken != nil && *c.RefreshToken != "" {
				updated, err := s.RefreshToken(ctx, tenantID, id)
				if err == nil {
					tokenPtr, _ := s.DecryptSecret(updated.BearerToken)
					if tokenPtr != nil && *tokenPtr != "" {
						return ResolveResponse{Header: "Authorization", Value: "Bearer " + *tokenPtr}, nil
					}
				}
			}
		}

		clientSecretPtr, _ := s.DecryptSecret(c.ClientSecret)
		c.ClientSecret = clientSecretPtr
		token, err := s.fetchOrCachedToken(ctx, id, c)
		if err != nil {
			return ResolveResponse{}, err
		}
		return ResolveResponse{Header: "Authorization", Value: "Bearer " + token}, nil

	case AuthTypeBearerToken:
		bearerTokenPtr, _ := s.DecryptSecret(c.BearerToken)
		val := ""
		if bearerTokenPtr != nil {
			val = *bearerTokenPtr
		}
		return ResolveResponse{Header: "Authorization", Value: "Bearer " + val}, nil

	case AuthTypeAPIKey:
		apiKeyValuePtr, _ := s.DecryptSecret(c.APIKeyValue)
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
		return ResolveResponse{Header: header, Value: val, Location: normalizeAPIKeyLocation(c.APIKeyLocation)}, nil

	case AuthTypeBasicAuth:
		passwordPtr, _ := s.DecryptSecret(c.Password)
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

	clientSecretPtr, _ := s.DecryptSecret(c.ClientSecret)
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

	clientSecretPtr, _ := s.DecryptSecret(c.ClientSecret)
	refreshTokenPtr, _ := s.DecryptSecret(c.RefreshToken)

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
