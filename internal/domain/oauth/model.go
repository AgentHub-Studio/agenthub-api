// Package oauth provides OAuth credential management for tenant-scoped HTTP integrations.
package oauth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an OAuthCredential is not found.
var ErrNotFound = errors.New("oauth credential not found")

// ErrValidation is returned when a CreateRequest fails server-side
// validation (empty name, unknown authType, etc). Without it the
// service silently accepted credentials with empty name/authType
// that were useless but persisted forever, polluting the tenant.
var ErrValidation = errors.New("oauth credential validation failed")

// AuthType represents the authentication mechanism for an OAuth credential.
type AuthType string

const (
	AuthTypeOAuth2ClientCredentials  AuthType = "OAUTH2_CLIENT_CREDENTIALS"
	AuthTypeOAuth2AuthorizationCode AuthType = "OAUTH2_AUTHORIZATION_CODE"
	AuthTypeAPIKey                  AuthType = "API_KEY"
	AuthTypeBearerToken             AuthType = "BEARER_TOKEN"
	AuthTypeBasicAuth               AuthType = "BASIC_AUTH"
)

// APIKeyLocation indicates whether an API key is sent as an HTTP header or a query parameter.
type APIKeyLocation string

const (
	APIKeyLocationHeader APIKeyLocation = "HEADER"
	APIKeyLocationQuery  APIKeyLocation = "QUERY"
)

// OAuthCredential stores credentials for outbound HTTP authentication.
type OAuthCredential struct {
	ID           uuid.UUID `db:"id"`
	Name         string    `db:"name"`
	AuthType     AuthType  `db:"auth_type"`
	TokenURL     *string    `db:"token_url"`
	ClientID     *string    `db:"client_id"`
	ClientSecret *string    `db:"client_secret"`
	Scopes       *string    `db:"scopes"`
	APIKeyHeader   *string        `db:"api_key_header"`
	APIKeyValue    *string        `db:"api_key_value"`
	APIKeyLocation APIKeyLocation `db:"api_key_location"`
	BearerToken  *string    `db:"bearer_token"`
	Username     *string    `db:"username"`
	Password     *string    `db:"password"`
	AuthURL      *string    `db:"auth_url"`
	RedirectURL  *string    `db:"redirect_url"`
	CodeVerifier *string    `db:"code_verifier"`
	RefreshToken *string    `db:"refresh_token"`
	ExpiresAt    *time.Time `db:"expires_at"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// OAuthCredentialResponse is the public DTO for OAuthCredential.
// Secret fields are zeroed by ResponseFrom.
type OAuthCredentialResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	AuthType     AuthType  `json:"authType"`
	TokenURL     *string    `json:"tokenUrl"`
	ClientID     *string    `json:"clientId"`
	ClientSecret *string    `json:"clientSecret"`
	Scopes       *string    `json:"scopes"`
	APIKeyHeader   *string        `json:"apiKeyHeader"`
	APIKeyValue    *string        `json:"apiKeyValue"`
	APIKeyLocation APIKeyLocation `json:"apiKeyLocation"`
	BearerToken    *string        `json:"bearerToken"`
	Username       *string        `json:"username"`
	Password       *string        `json:"password"`
	AuthURL        *string        `json:"authUrl"`
	RedirectURL    *string        `json:"redirectUrl"`
	CodeVerifier   *string        `json:"codeVerifier"`
	RefreshToken   *string        `json:"refreshToken"`
	ExpiresAt      *time.Time     `json:"expiresAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// ResponseFrom converts an OAuthCredential to its public DTO, masking all secret fields.
// Secret fields are replaced with "***" when they are non-empty.
func ResponseFrom(c OAuthCredential) OAuthCredentialResponse {
	r := OAuthCredentialResponse{
		ID:           c.ID,
		Name:         c.Name,
		AuthType:     c.AuthType,
		TokenURL:     c.TokenURL,
		ClientID:     c.ClientID,
		Scopes:       c.Scopes,
		APIKeyHeader:   c.APIKeyHeader,
		APIKeyLocation: c.APIKeyLocation,
		Username:       c.Username,
		AuthURL:      c.AuthURL,
		RedirectURL:  c.RedirectURL,
		ExpiresAt:    c.ExpiresAt,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
	r.ClientSecret = maskSecret(c.ClientSecret)
	r.APIKeyValue = maskSecret(c.APIKeyValue)
	r.BearerToken = maskSecret(c.BearerToken)
	r.Password = maskSecret(c.Password)
	r.RefreshToken = maskSecret(c.RefreshToken)
	return r
}

// maskSecret returns "***" when s is non-empty, preserving empty or nil as-is.
func maskSecret(s *string) *string {
	if s != nil && *s != "" {
		masked := "***"
		return &masked
	}
	return s
}

// CreateRequest is the payload for creating an OAuthCredential.
type CreateRequest struct {
	Name         string   `json:"name"`
	AuthType     AuthType `json:"authType"`
	TokenURL     *string  `json:"tokenUrl"`
	ClientID     *string  `json:"clientId"`
	ClientSecret *string  `json:"clientSecret"`
	Scopes       *string  `json:"scopes"`
	APIKeyHeader   *string        `json:"apiKeyHeader"`
	APIKeyValue    *string        `json:"apiKeyValue"`
	APIKeyLocation APIKeyLocation `json:"apiKeyLocation"`
	BearerToken  *string  `json:"bearerToken"`
	Username     *string  `json:"username"`
	Password     *string  `json:"password"`
	AuthURL      *string  `json:"authUrl"`
	RedirectURL  *string  `json:"redirectUrl"`
	CodeVerifier *string  `json:"codeVerifier"`
}

// ResolveResponse is the result of resolving an auth header.
// Location indicates whether the credential should be applied as an HTTP
// header (default) or a URL query parameter — relevant for API_KEY auth.
type ResolveResponse struct {
	Header   string         `json:"header"`
	Value    string         `json:"value"`
	Location APIKeyLocation `json:"location,omitempty"`
}

