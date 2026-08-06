package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// InstallRepository is the persistence interface for installations.
type InstallRepository interface {
	FindByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) ([]Installation, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (Installation, error)
	Create(ctx context.Context, i Installation) (Installation, error)
	Uninstall(ctx context.Context, id uuid.UUID) error
	SetHydrated(ctx context.Context, id uuid.UUID) error
}

// PackageReader loads a registry package before marketplace installation.
type PackageReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (pkg.Package, error)
}

// Service implements business logic for marketplace installations.
type Service struct {
	repo   InstallRepository
	pkgRdr PackageReader
}

// NewService creates a new Service.
func NewService(repo InstallRepository) *Service {
	return &Service{repo: repo}
}

// WithPackageReader wires a registry package reader.
func (s *Service) WithPackageReader(r PackageReader) *Service {
	s.pkgRdr = r
	return s
}

// Install installs a package for the given tenant.
func (s *Service) Install(ctx context.Context, tenantID string, req InstallRequest) (InstallResponse, error) {
	if req.PackageID == uuid.Nil {
		return InstallResponse{}, fmt.Errorf("installation: packageId is required")
	}
	if req.PackageVersion == "" {
		return InstallResponse{}, fmt.Errorf("installation: packageVersion is required")
	}
	if s.pkgRdr == nil {
		return InstallResponse{}, fmt.Errorf("installation: package reader is not configured")
	}
	p, err := s.pkgRdr.GetByID(ctx, req.PackageID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return InstallResponse{}, ErrPackageUnavailable
		}
		return InstallResponse{}, fmt.Errorf("installation: packageId not found")
	}
	if p.Visibility != pkg.PackageVisibilityPublic {
		return InstallResponse{}, ErrPackageUnavailable
	}
	i := Installation{
		ID:             uuid.New(),
		TenantID:       tenantID,
		PackageID:      req.PackageID,
		PackageVersion: req.PackageVersion,
		Status:         InstallStatusInstalled,
	}
	created, err := s.repo.Create(ctx, i)
	if err != nil {
		return InstallResponse{}, fmt.Errorf("installation: create: %w", err)
	}
	return ResponseFrom(created), nil
}

// Uninstall marks an installation as uninstalled.
func (s *Service) Uninstall(ctx context.Context, id uuid.UUID, tenantID string) error {
	i, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if i.TenantID != tenantID {
		return fmt.Errorf("installation: forbidden")
	}
	return s.repo.Uninstall(ctx, id)
}

// ListByTenant returns paginated installations for a tenant.
func (s *Service) ListByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[InstallResponse], error) {
	installations, total, err := s.repo.FindByTenant(ctx, tenantID, req)
	if err != nil {
		return pagination.Page[InstallResponse]{}, fmt.Errorf("installation: list by tenant: %w", err)
	}
	responses := make([]InstallResponse, len(installations))
	for i, inst := range installations {
		responses[i] = ResponseFrom(inst)
	}
	return pagination.NewPage(responses, total, req), nil
}
