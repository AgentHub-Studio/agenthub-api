package dependency

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
)

// Service implements business logic for package dependencies.
type Service struct {
	repo    *Repository
	pkgRepo *pkg.Repository
}

// NewService creates a new dependency Service.
func NewService(repo *Repository, pkgRepo *pkg.Repository) *Service {
	return &Service{repo: repo, pkgRepo: pkgRepo}
}

// List returns all direct dependencies of a package.
func (s *Service) List(ctx context.Context, packageID uuid.UUID) ([]DependencyResponse, error) {
	deps, err := s.repo.ListByPackage(ctx, packageID)
	if err != nil {
		return nil, fmt.Errorf("service: list dependencies: %w", err)
	}
	responses := make([]DependencyResponse, 0, len(deps))
	for _, d := range deps {
		responses = append(responses, ResponseFrom(d))
	}
	return responses, nil
}

// Add adds a dependency to a package. Caller must own the package.
func (s *Service) Add(ctx context.Context, packageID uuid.UUID, req AddDependencyRequest, tenantID string) (DependencyResponse, error) {
	if strings.TrimSpace(req.VersionConstraint) == "" {
		return DependencyResponse{}, &ValidationError{Field: "versionConstraint", Message: "version constraint is required"}
	}
	if req.DependencyID == packageID {
		return DependencyResponse{}, &ValidationError{Field: "dependencyId", Message: "a package cannot depend on itself"}
	}

	parent, err := s.pkgRepo.GetByID(ctx, packageID)
	if err != nil {
		return DependencyResponse{}, err
	}
	if parent.AuthorTenantID != tenantID {
		return DependencyResponse{}, &ForbiddenError{Message: "not the package owner"}
	}

	// Ensure the dependency package exists.
	if _, err := s.pkgRepo.GetByID(ctx, req.DependencyID); err != nil {
		return DependencyResponse{}, fmt.Errorf("dependency target not found: %w", err)
	}

	d := PackageDependency{
		PackageID:         packageID,
		DependencyID:      req.DependencyID,
		VersionConstraint: req.VersionConstraint,
	}

	created, err := s.repo.Create(ctx, d)
	if err != nil {
		return DependencyResponse{}, fmt.Errorf("service: add dependency: %w", err)
	}
	return ResponseFrom(created), nil
}

// Remove deletes a dependency edge. Caller must own the package.
func (s *Service) Remove(ctx context.Context, packageID, depID uuid.UUID, tenantID string) error {
	parent, err := s.pkgRepo.GetByID(ctx, packageID)
	if err != nil {
		return err
	}
	if parent.AuthorTenantID != tenantID {
		return &ForbiddenError{Message: "not the package owner"}
	}
	return s.repo.Delete(ctx, packageID, depID)
}

// Resolve performs a BFS traversal of the dependency graph starting from packageID,
// returning the full transitive dependency tree without cycles.
func (s *Service) Resolve(ctx context.Context, packageID uuid.UUID) (ResolvedDependency, error) {
	visited := make(map[uuid.UUID]bool)
	return s.resolveNode(ctx, packageID, "", visited)
}

func (s *Service) resolveNode(ctx context.Context, packageID uuid.UUID, constraint string, visited map[uuid.UUID]bool) (ResolvedDependency, error) {
	name, slug, err := s.repo.GetPackageName(ctx, packageID)
	if err != nil {
		return ResolvedDependency{}, fmt.Errorf("resolve: get package %s: %w", packageID, err)
	}

	node := ResolvedDependency{
		PackageID:         packageID,
		Name:              name,
		Slug:              slug,
		VersionConstraint: constraint,
	}

	// Cycle detection: if already visited, return node without expanding.
	if visited[packageID] {
		return node, nil
	}
	visited[packageID] = true

	deps, err := s.repo.ListByPackage(ctx, packageID)
	if err != nil {
		return ResolvedDependency{}, fmt.Errorf("resolve: list deps of %s: %w", packageID, err)
	}

	for _, d := range deps {
		child, err := s.resolveNode(ctx, d.DependencyID, d.VersionConstraint, visited)
		if err != nil {
			return ResolvedDependency{}, err
		}
		node.Dependencies = append(node.Dependencies, child)
	}

	return node, nil
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
