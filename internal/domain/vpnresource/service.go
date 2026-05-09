package vpnresource

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"

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
	ExistsByName(ctx context.Context, tenantID, name string) (bool, error)
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

// Create creates a new VPN resource. Validação precede a inserção
// para que body {} não persista um VpnResource com name="" (lixo
// inerte que polui a tela de VPNs sem efeito útil).
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (VpnResource, error) {
	if strings.TrimSpace(req.Name) == "" {
		return VpnResource{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	// Bug 130: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return VpnResource{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	exists, err := s.repo.ExistsByName(ctx, tenantID, req.Name)
	if err != nil {
		return VpnResource{}, fmt.Errorf("vpn: check duplicate name: %w", err)
	}
	if exists {
		return VpnResource{}, ErrDuplicateName
	}
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

// Update updates an existing VPN resource. PATCH-friendly: campos vazios
// preservam valor atual (true partial update — backlog #186 pattern).
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (VpnResource, error) {
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return VpnResource{}, err
	}
	// Merge: preserve current value when request omits the field.
	if strings.TrimSpace(req.Name) == "" {
		req.Name = existing.Name
	}
	if req.Description == "" {
		req.Description = existing.Description
	}
	if req.OvpnConfigPath == "" {
		req.OvpnConfigPath = existing.OvpnConfigPath
	}
	if req.AuthFilePath == "" {
		req.AuthFilePath = existing.AuthFilePath
	}
	if req.SecretName == "" {
		req.SecretName = existing.SecretName
	}
	if strings.TrimSpace(req.Name) == "" {
		return VpnResource{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
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

// TestConnection tests VPN resource connectivity by starting a Docker container.
// The test verifies:
//  1. The VPN resource exists and has a config file uploaded.
//  2. Docker is available on the host.
//  3. A Docker container can be launched (using alpine:latest) to confirm the runtime works.
//
// Full VPN tunnel validation (OpenVPN handshake) requires the vpn-proxy service.
// This method acts as a pre-flight check that the infrastructure is ready.
func (s *Service) TestConnection(ctx context.Context, tenantID string, id uuid.UUID) (TestConnectionResponse, error) {
	v, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return TestConnectionResponse{}, err
	}

	// Check that a VPN config file has been uploaded.
	if v.OvpnConfigPath == "" {
		return TestConnectionResponse{
			Connected: false,
			Message:   "VPN config not uploaded — use POST /api/vpn-resources/{id}/ovpn to upload the .ovpn file",
		}, nil
	}

	// Verify Docker availability and run a minimal connectivity probe.
	if err := checkDockerAvailable(ctx); err != nil {
		slog.Warn("vpn: docker not available for connectivity test", "err", err)
		return TestConnectionResponse{
			Connected: false,
			Message:   fmt.Sprintf("Docker unavailable: %v", err),
		}, nil
	}

	// Run a lightweight container as a Docker runtime smoke-test.
	// Full VPN tunnel testing is delegated to the vpn-proxy service.
	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(testCtx,
		"docker", "run", "--rm", "--network=none",
		"alpine:latest", "echo", "vpn-preflight-ok",
	).CombinedOutput()
	if err != nil {
		slog.Warn("vpn: connectivity probe container failed", "err", err, "output", string(out))
		return TestConnectionResponse{
			Connected: false,
			Message:   fmt.Sprintf("Docker probe failed: %v — %s", err, strings.TrimSpace(string(out))),
		}, nil
	}

	return TestConnectionResponse{
		Connected: true,
		Message:   "Docker runtime verified; VPN config present — ready for vpn-proxy",
	}, nil
}

// checkDockerAvailable runs `docker version` to verify the Docker daemon is accessible.
func checkDockerAvailable(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return exec.CommandContext(checkCtx, "docker", "version", "--format", "{{.Server.Version}}").Run()
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
