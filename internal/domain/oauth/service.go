package oauth

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"

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

// Service implements business logic for OAuth credentials.
type Service struct {
	repo CredentialRepository
}

// NewService creates a new Service.
func NewService(repo CredentialRepository) *Service {
	return &Service{repo: repo}
}

// ListAll returns a paginated list of OAuth credentials for the tenant.
func (s *Service) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]OAuthCredential, int, error) {
	return s.repo.ListAll(ctx, tenantID, pr)
}

// GetByID retrieves an OAuth credential by ID.
func (s *Service) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// Create creates a new OAuth credential.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (OAuthCredential, error) {
	c := OAuthCredential{
		Name:         req.Name,
		AuthType:     req.AuthType,
		TokenURL:     req.TokenURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		Scopes:       req.Scopes,
		APIKeyHeader: req.APIKeyHeader,
		APIKeyValue:  req.APIKeyValue,
		BearerToken:  req.BearerToken,
		Username:     req.Username,
		Password:     req.Password,
	}
	return s.repo.Create(ctx, tenantID, c)
}

// Update updates an existing OAuth credential.
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (OAuthCredential, error) {
	c := OAuthCredential{
		Name:         req.Name,
		AuthType:     req.AuthType,
		TokenURL:     req.TokenURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		Scopes:       req.Scopes,
		APIKeyHeader: req.APIKeyHeader,
		APIKeyValue:  req.APIKeyValue,
		BearerToken:  req.BearerToken,
		Username:     req.Username,
		Password:     req.Password,
	}
	return s.repo.Update(ctx, tenantID, id, c)
}

// Delete removes an OAuth credential.
func (s *Service) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}

// ResolveAuthHeader resolves the credential to an HTTP Authorization header value.
func (s *Service) ResolveAuthHeader(ctx context.Context, tenantID string, id uuid.UUID) (ResolveResponse, error) {
	c, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return ResolveResponse{}, err
	}

	switch c.AuthType {
	case AuthTypeOAuth2ClientCredentials, AuthTypeBearerToken:
		return ResolveResponse{Header: "Authorization", Value: "Bearer " + c.BearerToken}, nil
	case AuthTypeAPIKey:
		header := c.APIKeyHeader
		if header == "" {
			header = "X-API-Key"
		}
		return ResolveResponse{Header: header, Value: c.APIKeyValue}, nil
	case AuthTypeBasicAuth:
		encoded := base64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password))
		return ResolveResponse{Header: "Authorization", Value: "Basic " + encoded}, nil
	default:
		return ResolveResponse{}, fmt.Errorf("oauth: unsupported auth type: %s", c.AuthType)
	}
}
