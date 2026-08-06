package pkg

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// packageRepo defines the data access methods required by Service.
type packageRepo interface {
	ListPublic(ctx context.Context, req pagination.PageRequest) ([]Package, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (Package, error)
	GetBySlug(ctx context.Context, slug string) (Package, error)
	ListByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) ([]Package, int64, error)
	Create(ctx context.Context, p Package) (Package, error)
	Update(ctx context.Context, id uuid.UUID, name, description, visibility string, tags []string) (Package, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Search(ctx context.Context, query string, pkgType *string, req pagination.PageRequest) ([]Package, int64, error)
}

// Service implements business logic for the package registry.
type Service struct {
	repo packageRepo
}

// NewService creates a new package Service.
func NewService(repo packageRepo) *Service {
	return &Service{repo: repo}
}

// ListPublic returns a paginated page of PUBLIC packages.
func (s *Service) ListPublic(ctx context.Context, req pagination.PageRequest) (pagination.Page[PackageResponse], error) {
	pkgs, total, err := s.repo.ListPublic(ctx, req)
	if err != nil {
		return pagination.Page[PackageResponse]{}, fmt.Errorf("service: list public packages: %w", err)
	}
	responses := make([]PackageResponse, 0, len(pkgs))
	for _, p := range pkgs {
		responses = append(responses, ResponseFrom(p))
	}
	return pagination.NewPage(responses, total, req), nil
}

// GetByID returns a package by its UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (PackageResponse, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return PackageResponse{}, err
	}
	return ResponseFrom(p), nil
}

// GetBySlug returns a package by slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (PackageResponse, error) {
	p, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return PackageResponse{}, err
	}
	return ResponseFrom(p), nil
}

// GetAccessibleByID returns a PUBLIC package to any caller and a PRIVATE
// package only to its author. Hidden packages deliberately use ErrNotFound so
// callers cannot distinguish them from unknown package IDs.
func (s *Service) GetAccessibleByID(ctx context.Context, id uuid.UUID, tenantID string) (PackageResponse, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return PackageResponse{}, err
	}
	if !isAccessibleTo(p, tenantID) {
		return PackageResponse{}, ErrNotFound
	}
	return ResponseFrom(p), nil
}

// GetAccessibleBySlug returns a package only when it is public or belongs to
// the authenticated tenant. It shares the non-enumerating not-found behavior
// of GetAccessibleByID.
func (s *Service) GetAccessibleBySlug(ctx context.Context, slug, tenantID string) (PackageResponse, error) {
	p, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return PackageResponse{}, err
	}
	if !isAccessibleTo(p, tenantID) {
		return PackageResponse{}, ErrNotFound
	}
	return ResponseFrom(p), nil
}

func isAccessibleTo(p Package, tenantID string) bool {
	return p.Visibility == PackageVisibilityPublic || (tenantID != "" && p.AuthorTenantID == tenantID)
}

// ListByTenant returns packages owned by the given tenant.
func (s *Service) ListByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[PackageResponse], error) {
	pkgs, total, err := s.repo.ListByTenant(ctx, tenantID, req)
	if err != nil {
		return pagination.Page[PackageResponse]{}, fmt.Errorf("service: list packages by tenant: %w", err)
	}
	responses := make([]PackageResponse, 0, len(pkgs))
	for _, p := range pkgs {
		responses = append(responses, ResponseFrom(p))
	}
	return pagination.NewPage(responses, total, req), nil
}

// Create validates and creates a new package.
func (s *Service) Create(ctx context.Context, req CreatePackageRequest, tenantID string) (PackageResponse, error) {
	// Bug 183: strip HTML do name (XSS prevention).
	req.Name = sanitize.StripHTML(req.Name)
	if err := validateCreate(req); err != nil {
		return PackageResponse{}, err
	}

	visibility := PackageVisibilityPrivate
	if strings.ToUpper(req.Visibility) == string(PackageVisibilityPublic) {
		visibility = PackageVisibilityPublic
	}

	p := Package{
		Name:           req.Name,
		Slug:           req.Slug,
		Description:    req.Description,
		Tags:           normalizeTags(req.Tags),
		Type:           PackageType(strings.ToUpper(req.Type)),
		Visibility:     visibility,
		AuthorTenantID: tenantID,
	}

	created, err := s.repo.Create(ctx, p)
	if err != nil {
		return PackageResponse{}, fmt.Errorf("service: create package: %w", err)
	}
	return ResponseFrom(created), nil
}

// Update applies partial updates to a package. Caller must own the package.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdatePackageRequest, tenantID string) (PackageResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return PackageResponse{}, err
	}
	if existing.AuthorTenantID != tenantID {
		return PackageResponse{}, &ForbiddenError{Message: "not the package owner"}
	}

	name := existing.Name
	description := existing.Description
	visibility := string(existing.Visibility)
	tags := existing.Tags

	if req.Name != nil {
		// Bug 183: strip HTML (XSS prevention).
		name = sanitize.StripHTML(*req.Name)
	}
	if req.Description != nil {
		description = *req.Description
	}
	if req.Visibility != nil {
		visibility = strings.ToUpper(*req.Visibility)
	}
	if req.Tags != nil {
		tags = normalizeTags(*req.Tags)
	}

	updated, err := s.repo.Update(ctx, id, name, description, visibility, tags)
	if err != nil {
		return PackageResponse{}, fmt.Errorf("service: update package: %w", err)
	}
	return ResponseFrom(updated), nil
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

// Delete removes a package. Caller must own the package.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, tenantID string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.AuthorTenantID != tenantID {
		return &ForbiddenError{Message: "not the package owner"}
	}
	return s.repo.Delete(ctx, id)
}

// Search performs a text search across PUBLIC packages by name, slug, and description.
// An optional type filter restricts results to a specific package type.
func (s *Service) Search(ctx context.Context, query string, pkgType *string, req pagination.PageRequest) (pagination.Page[PackageResponse], error) {
	pkgs, total, err := s.repo.Search(ctx, query, pkgType, req)
	if err != nil {
		return pagination.Page[PackageResponse]{}, fmt.Errorf("service: search packages: %w", err)
	}
	responses := make([]PackageResponse, 0, len(pkgs))
	for _, p := range pkgs {
		responses = append(responses, ResponseFrom(p))
	}
	return pagination.NewPage(responses, total, req), nil
}

func validateCreate(req CreatePackageRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	// Bug 131: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return &ValidationError{Field: "name", Message: fmt.Sprintf("name exceeds maximum length of 255 chars (got %d)", len(req.Name))}
	}
	if strings.TrimSpace(req.Slug) == "" {
		return &ValidationError{Field: "slug", Message: "slug is required"}
	}
	if !slugRegex.MatchString(req.Slug) {
		return &ValidationError{Field: "slug", Message: "slug must be lowercase alphanumeric with hyphens"}
	}
	validTypes := map[string]bool{
		"AGENT": true, "SKILL": true, "TOOL": true, "KNOWLEDGE_BASE": true,
	}
	if !validTypes[strings.ToUpper(req.Type)] {
		return &ValidationError{Field: "type", Message: "type must be AGENT, SKILL, TOOL or KNOWLEDGE_BASE"}
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
