package vpnresource

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// StorageClient abstracts object-storage uploads for VPN config files.
type StorageClient interface {
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)
}

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
	repo    VpnRepository
	storage StorageClient
}

// NewService creates a new Service with a noop storage client.
func NewService(repo VpnRepository) *Service {
	return &Service{repo: repo, storage: &noopStorageClient{}}
}

// NewServiceWithStorage creates a new Service with a custom storage client.
func NewServiceWithStorage(repo VpnRepository, storage StorageClient) *Service {
	return &Service{repo: repo, storage: storage}
}

// noopStorageClient accepts uploads silently — used when MinIO is not configured.
type noopStorageClient struct{}

func (n *noopStorageClient) Upload(_ context.Context, key string, _ io.Reader, _ int64, _ string) (string, error) {
	return key, nil
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

// UploadOvpnConfig validates and uploads a .ovpn config file to object storage,
// then updates the VpnResource.OvpnConfigPath.
func (s *Service) UploadOvpnConfig(ctx context.Context, tenantID string, id uuid.UUID, r io.Reader, size int64) (VpnResource, error) {
	v, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return VpnResource{}, err
	}

	// Buffer entire content for validation (max 1 MB).
	const maxOvpnSize = 1 << 20
	if size > maxOvpnSize {
		return VpnResource{}, fmt.Errorf("vpn: ovpn config exceeds maximum size of 1 MB")
	}

	buf, err := io.ReadAll(io.LimitReader(r, maxOvpnSize+1))
	if err != nil {
		return VpnResource{}, fmt.Errorf("vpn: read ovpn config: %w", err)
	}
	if err := validateOvpnContent(string(buf)); err != nil {
		return VpnResource{}, err
	}

	key := fmt.Sprintf("vpn/%s/%s/config.ovpn", tenantID, id)
	storedKey, err := s.storage.Upload(ctx, key, strings.NewReader(string(buf)), int64(len(buf)), "application/octet-stream")
	if err != nil {
		return VpnResource{}, fmt.Errorf("vpn: upload ovpn config: %w", err)
	}

	v.OvpnConfigPath = storedKey
	return s.repo.Update(ctx, tenantID, id, VpnResource{
		Name:           v.Name,
		Description:    v.Description,
		Enabled:        v.Enabled,
		OvpnConfigPath: storedKey,
		AuthFilePath:   v.AuthFilePath,
		SecretName:     v.SecretName,
	})
}

// UploadAuthFile uploads a VPN auth file (username/password) to object storage,
// then updates the VpnResource.AuthFilePath.
func (s *Service) UploadAuthFile(ctx context.Context, tenantID string, id uuid.UUID, r io.Reader, size int64) (VpnResource, error) {
	v, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return VpnResource{}, err
	}

	const maxAuthSize = 4096
	if size > maxAuthSize {
		return VpnResource{}, fmt.Errorf("vpn: auth file exceeds maximum size of 4 KB")
	}

	buf, err := io.ReadAll(io.LimitReader(r, maxAuthSize+1))
	if err != nil {
		return VpnResource{}, fmt.Errorf("vpn: read auth file: %w", err)
	}

	key := fmt.Sprintf("vpn/%s/%s/auth.txt", tenantID, id)
	storedKey, err := s.storage.Upload(ctx, key, strings.NewReader(string(buf)), int64(len(buf)), "text/plain")
	if err != nil {
		return VpnResource{}, fmt.Errorf("vpn: upload auth file: %w", err)
	}

	v.AuthFilePath = storedKey
	return s.repo.Update(ctx, tenantID, id, VpnResource{
		Name:           v.Name,
		Description:    v.Description,
		Enabled:        v.Enabled,
		OvpnConfigPath: v.OvpnConfigPath,
		AuthFilePath:   storedKey,
		SecretName:     v.SecretName,
	})
}

// validateOvpnContent performs basic structural validation of an OpenVPN config file.
// A valid file must contain "remote" and "dev" directives.
func validateOvpnContent(content string) error {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "remote ") {
		return fmt.Errorf("vpn: invalid .ovpn file: missing 'remote' directive")
	}
	if !strings.Contains(lower, "dev ") {
		return fmt.Errorf("vpn: invalid .ovpn file: missing 'dev' directive")
	}
	return nil
}
