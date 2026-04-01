package version

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
)

// semverRegex matches a simple semver string: major.minor.patch
var semverRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Service implements business logic for package versions.
type Service struct {
	repo    *Repository
	pkgRepo *pkg.Repository
}

// NewService creates a new version Service.
func NewService(repo *Repository, pkgRepo *pkg.Repository) *Service {
	return &Service{repo: repo, pkgRepo: pkgRepo}
}

// ListByPackage returns all versions for a given package.
func (s *Service) ListByPackage(ctx context.Context, packageID uuid.UUID) ([]VersionResponse, error) {
	versions, err := s.repo.ListByPackage(ctx, packageID)
	if err != nil {
		return nil, fmt.Errorf("service: list versions: %w", err)
	}
	responses := make([]VersionResponse, 0, len(versions))
	for _, v := range versions {
		responses = append(responses, ResponseFrom(v))
	}
	return responses, nil
}

// GetByVersion returns a specific version.
func (s *Service) GetByVersion(ctx context.Context, packageID uuid.UUID, versionStr string) (VersionResponse, error) {
	v, err := s.repo.GetByVersion(ctx, packageID, versionStr)
	if err != nil {
		return VersionResponse{}, err
	}
	return ResponseFrom(v), nil
}

// Publish creates a new version and updates latest_version on the package.
// Caller must be the package owner.
func (s *Service) Publish(ctx context.Context, packageID uuid.UUID, req PublishVersionRequest, tenantID string) (VersionResponse, error) {
	if err := validatePublish(req); err != nil {
		return VersionResponse{}, err
	}

	parent, err := s.pkgRepo.GetByID(ctx, packageID)
	if err != nil {
		return VersionResponse{}, err
	}
	if parent.AuthorTenantID != tenantID {
		return VersionResponse{}, &ForbiddenError{Message: "not the package owner"}
	}

	v := PackageVersion{
		PackageID:   packageID,
		Version:     req.Version,
		Changelog:   req.Changelog,
		StoragePath: req.StoragePath,
		Checksum:    req.Checksum,
		PublishedBy: tenantID,
	}

	created, err := s.repo.Create(ctx, v)
	if err != nil {
		return VersionResponse{}, fmt.Errorf("service: publish version: %w", err)
	}

	// Update latest_version on the package registry entry.
	if err := s.pkgRepo.UpdateLatestVersion(ctx, packageID, created.Version); err != nil {
		// Non-fatal: best effort
		_ = err
	}

	return ResponseFrom(created), nil
}

// Delete removes a version. Caller must be the package owner.
func (s *Service) Delete(ctx context.Context, packageID uuid.UUID, versionStr string, tenantID string) error {
	parent, err := s.pkgRepo.GetByID(ctx, packageID)
	if err != nil {
		return err
	}
	if parent.AuthorTenantID != tenantID {
		return &ForbiddenError{Message: "not the package owner"}
	}
	return s.repo.Delete(ctx, packageID, versionStr)
}

func validatePublish(req PublishVersionRequest) error {
	if strings.TrimSpace(req.Version) == "" {
		return &ValidationError{Field: "version", Message: "version is required"}
	}
	if !semverRegex.MatchString(req.Version) {
		return &ValidationError{Field: "version", Message: "version must follow semver (e.g. 1.2.3)"}
	}
	return nil
}

// ValidationError represents an input validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s — %s", e.Field, e.Message)
}

// ForbiddenError represents an authorization failure.
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string {
	return e.Message
}
