package tenant

import (
	"context"
	"fmt"
	"regexp"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

var slugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)

// Service defines business logic operations for Tenant.
type Service interface {
	Create(ctx context.Context, req CreateTenantRequest) (TenantResponse, error)
	GetByID(ctx context.Context, id string) (TenantResponse, error)
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[TenantResponse], error)
	Exists(ctx context.Context, id string) (bool, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
}

type service struct {
	repo               Repository
	provisioningClient ProvisioningClient
}

// ProvisioningClient is a placeholder interface for Keycloak realm provisioning.
// Wire a real implementation when Keycloak integration is ready.
type ProvisioningClient interface {
	ProvisionRealm(ctx context.Context, tenantID string, tenantName string) error
}

// NewService creates a new tenant Service.
func NewService(repo Repository, pc ProvisioningClient) Service {
	return &service{repo: repo, provisioningClient: pc}
}

func (s *service) Create(ctx context.Context, req CreateTenantRequest) (TenantResponse, error) {
	if req.ID == "" {
		return TenantResponse{}, fmt.Errorf("id is required")
	}
	if req.Name == "" {
		return TenantResponse{}, fmt.Errorf("name is required")
	}
	if !slugRegexp.MatchString(req.ID) {
		return TenantResponse{}, fmt.Errorf("id must be a kebab-case slug (^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$)")
	}

	t := Tenant{
		ID:     req.ID,
		Name:   req.Name,
		Status: StatusActive,
	}
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return TenantResponse{}, err
	}

	// Attempt Keycloak provisioning; on failure mark status but do not rollback.
	if s.provisioningClient != nil {
		if pErr := s.provisioningClient.ProvisionRealm(ctx, created.ID, created.Name); pErr != nil {
			_ = s.repo.UpdateStatus(ctx, created.ID, StatusProvisioningFailed)
			created.Status = StatusProvisioningFailed
		}
	}

	return ResponseFrom(created), nil
}

func (s *service) GetByID(ctx context.Context, id string) (TenantResponse, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return TenantResponse{}, err
	}
	return ResponseFrom(t), nil
}

func (s *service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[TenantResponse], error) {
	tenants, total, err := s.repo.FindAll(ctx, req)
	if err != nil {
		return pagination.Page[TenantResponse]{}, err
	}
	responses := make([]TenantResponse, len(tenants))
	for i, t := range tenants {
		responses[i] = ResponseFrom(t)
	}
	return pagination.NewPage(responses, total, req), nil
}

func (s *service) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
}

func (s *service) UpdateStatus(ctx context.Context, id string, status Status) error {
	return s.repo.UpdateStatus(ctx, id, status)
}
