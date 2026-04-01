package vpnresource

import (
	"context"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// VpnRepository defines the persistence interface for VpnResource.
type VpnRepository interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]VpnResource, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (VpnResource, error)
	Create(ctx context.Context, tenantID string, v VpnResource) (VpnResource, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, v VpnResource) (VpnResource, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
}

// Service implements business logic for VPN resources.
type Service struct {
	repo VpnRepository
}

// NewService creates a new Service.
func NewService(repo VpnRepository) *Service {
	return &Service{repo: repo}
}

// ListAll returns a paginated list of VPN resources.
func (s *Service) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]VpnResource, int, error) {
	return s.repo.ListAll(ctx, tenantID, pr)
}

// GetByID retrieves a VPN resource by ID.
func (s *Service) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (VpnResource, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// Create creates a new VPN resource.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (VpnResource, error) {
	v := VpnResource{
		Name:           req.Name,
		Description:    req.Description,
		Enabled:        req.Enabled,
		OvpnConfigPath: req.OvpnConfigPath,
		AuthFilePath:   req.AuthFilePath,
		SecretName:     req.SecretName,
	}
	return s.repo.Create(ctx, tenantID, v)
}

// Update updates an existing VPN resource.
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (VpnResource, error) {
	v := VpnResource{
		Name:           req.Name,
		Description:    req.Description,
		Enabled:        req.Enabled,
		OvpnConfigPath: req.OvpnConfigPath,
		AuthFilePath:   req.AuthFilePath,
		SecretName:     req.SecretName,
	}
	return s.repo.Update(ctx, tenantID, id, v)
}

// Delete removes a VPN resource.
func (s *Service) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}

// TestConnection performs a noop connectivity test for the VPN resource.
func (s *Service) TestConnection(ctx context.Context, tenantID string, id uuid.UUID) (TestConnectionResponse, error) {
	// Verify resource exists before reporting connected.
	if _, err := s.repo.GetByID(ctx, tenantID, id); err != nil {
		return TestConnectionResponse{}, err
	}
	return TestConnectionResponse{Connected: true, Message: "noop"}, nil
}
