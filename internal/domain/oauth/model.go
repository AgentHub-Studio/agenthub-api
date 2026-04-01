// Package oauth provides OAuth credential management for tenant-scoped HTTP integrations.
package oauth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an OAuthCredential is not found.
var ErrNotFound = errors.New("oauth credential not found")

// AuthType represents the authentication mechanism for an OAuth credential.
type AuthType string

const (
	AuthTypeOAuth2ClientCredentials AuthType = "OAUTH2_CLIENT_CREDENTIALS"
	AuthTypeAPIKey                  AuthType = "API_KEY"
	AuthTypeBearerToken             AuthType = "BEARER_TOKEN"
	AuthTypeBasicAuth               AuthType = "BASIC_AUTH"
)

// OAuthCredential stores credentials for outbound HTTP authentication.
type OAuthCredential struct {
	ID           uuid.UUID `db:"id"`
	Name         string    `db:"name"`
	AuthType     AuthType  `db:"auth_type"`
	TokenURL     string    `db:"token_url"`
	ClientID     string    `db:"client_id"`
	ClientSecret string    `db:"client_secret"`
	Scopes       string    `db:"scopes"`
	APIKeyHeader string    `db:"api_key_header"`
	APIKeyValue  string    `db:"api_key_value"`
	BearerToken  string    `db:"bearer_token"`
	Username     string    `db:"username"`
	Password     string    `db:"password"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// OAuthCredentialResponse is the public DTO for OAuthCredential.
// Secret fields are zeroed by ResponseFrom.
type OAuthCredentialResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	AuthType     AuthType  `json:"authType"`
	TokenURL     string    `json:"tokenUrl"`
	ClientID     string    `json:"clientId"`
	ClientSecret string    `json:"clientSecret"`
	Scopes       string    `json:"scopes"`
	APIKeyHeader string    `json:"apiKeyHeader"`
	APIKeyValue  string    `json:"apiKeyValue"`
	BearerToken  string    `json:"bearerToken"`
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ResponseFrom converts an OAuthCredential to its public DTO, masking all secret fields.
func ResponseFrom(c OAuthCredential) OAuthCredentialResponse {
	r := OAuthCredentialResponse(c)
	r.ClientSecret = ""
	r.APIKeyValue = ""
	r.BearerToken = ""
	r.Password = ""
	return r
}

// CreateRequest is the payload for creating an OAuthCredential.
type CreateRequest struct {
	Name         string   `json:"name"`
	AuthType     AuthType `json:"authType"`
	TokenURL     string   `json:"tokenUrl"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	Scopes       string   `json:"scopes"`
	APIKeyHeader string   `json:"apiKeyHeader"`
	APIKeyValue  string   `json:"apiKeyValue"`
	BearerToken  string   `json:"bearerToken"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
}

// ResolveResponse is the result of resolving an auth header.
type ResolveResponse struct {
	Header string `json:"header"`
	Value  string `json:"value"`
}
