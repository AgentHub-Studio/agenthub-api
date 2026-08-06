package listing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

// PackageReader reads registry packages to bind a marketplace listing to its
// owning tenant.
type PackageReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (pkg.Package, error)
}

// Service implements business logic for marketplace listings.
type Service struct {
	repo     ListingRepository
	packages PackageReader
}

// NewService creates a new Service.
func NewService(repo ListingRepository, packages PackageReader) *Service {
	return &Service{repo: repo, packages: packages}
}

// ListAll returns all active listings paginated.
func (s *Service) ListAll(ctx context.Context, req pagination.PageRequest) (pagination.Page[ListingResponse], error) {
	listings, total, err := s.repo.FindAll(ctx, req)
	if err != nil {
		return pagination.Page[ListingResponse]{}, fmt.Errorf("listing: list all: %w", err)
	}
	return toPage(listings, total, req), nil
}

// ListByType returns listings filtered by type.
func (s *Service) ListByType(ctx context.Context, t PackageType, req pagination.PageRequest) (pagination.Page[ListingResponse], error) {
	listings, total, err := s.repo.FindByType(ctx, t, req)
	if err != nil {
		return pagination.Page[ListingResponse]{}, fmt.Errorf("listing: list by type: %w", err)
	}
	return toPage(listings, total, req), nil
}

// ListByCategory returns listings filtered by category.
func (s *Service) ListByCategory(ctx context.Context, cat string, req pagination.PageRequest) (pagination.Page[ListingResponse], error) {
	listings, total, err := s.repo.FindByCategory(ctx, cat, req)
	if err != nil {
		return pagination.Page[ListingResponse]{}, fmt.Errorf("listing: list by category: %w", err)
	}
	return toPage(listings, total, req), nil
}

// ListByTenant returns listings published by the given tenant.
func (s *Service) ListByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[ListingResponse], error) {
	listings, total, err := s.repo.FindByTenant(ctx, tenantID, req)
	if err != nil {
		return pagination.Page[ListingResponse]{}, fmt.Errorf("listing: list by tenant: %w", err)
	}
	return toPage(listings, total, req), nil
}

// GetByID returns a listing by UUID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (ListingResponse, error) {
	l, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ListingResponse{}, err
	}
	if err := s.ensurePackagePublic(ctx, l.PackageID); err != nil {
		return ListingResponse{}, err
	}
	return ResponseFrom(l), nil
}

// GetBySlug returns a listing by slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (ListingResponse, error) {
	l, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return ListingResponse{}, err
	}
	if err := s.ensurePackagePublic(ctx, l.PackageID); err != nil {
		return ListingResponse{}, err
	}
	return ResponseFrom(l), nil
}

// Create publishes a new marketplace listing.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateListingRequest) (ListingResponse, error) {
	if req.PackageID == uuid.Nil {
		return ListingResponse{}, fmt.Errorf("%w: packageId is required", ErrValidation)
	}
	// Bug 183: strip HTML do name (XSS prevention).
	req.Name = sanitize.StripHTML(req.Name)
	if req.Name == "" {
		return ListingResponse{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	// Bug 131: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return ListingResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	// Bug 143: type field era armazenado sem validação (silenciosamente
	// aceitando "INVALID" ou "" no banco). Gate enum + obrigatoriedade.
	if req.Type == "" {
		return ListingResponse{}, fmt.Errorf("%w: type is required (one of AGENT, SKILL, TOOL, KNOWLEDGE_BASE)", ErrValidation)
	}
	switch PackageType(req.Type) {
	case PackageTypeAgent, PackageTypeSkill, PackageTypeTool, PackageTypeKnowledgeBase:
	default:
		return ListingResponse{}, fmt.Errorf("%w: type must be one of AGENT, SKILL, TOOL, KNOWLEDGE_BASE (got %q)", ErrValidation, req.Type)
	}
	// Bug 159: cap description em 32KB.
	if len(req.Description) > 32000 {
		return ListingResponse{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	}
	if s.packages == nil {
		return ListingResponse{}, fmt.Errorf("listing: package reader is not configured")
	}
	p, err := s.packages.GetByID(ctx, req.PackageID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return ListingResponse{}, ErrPackageNotFound
		}
		return ListingResponse{}, fmt.Errorf("listing: get package: %w", err)
	}
	if p.Visibility != pkg.PackageVisibilityPublic {
		if p.AuthorTenantID != tenantID {
			return ListingResponse{}, ErrPackageNotFound
		}
		return ListingResponse{}, ErrPackageNotPublic
	}
	if p.AuthorTenantID != tenantID {
		return ListingResponse{}, ErrForbidden
	}
	slug := req.Slug
	if slug == "" {
		slug = toSlug(req.Name)
	}
	l := Listing{
		ID:          uuid.New(),
		TenantID:    tenantID,
		PackageID:   req.PackageID,
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		Type:        PackageType(req.Type),
		Category:    req.Category,
		Status:      StatusActive,
	}
	created, err := s.repo.Create(ctx, l)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ListingResponse{}, ErrDuplicateSlug
		}
		return ListingResponse{}, fmt.Errorf("listing: create: %w", err)
	}
	return ResponseFrom(created), nil
}

func (s *Service) ensurePackagePublic(ctx context.Context, packageID uuid.UUID) error {
	if s.packages == nil {
		return fmt.Errorf("listing: package reader is not configured")
	}
	p, err := s.packages.GetByID(ctx, packageID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("listing: get package: %w", err)
	}
	if p.Visibility != pkg.PackageVisibilityPublic {
		return ErrNotFound
	}
	return nil
}

// Update updates mutable fields of a listing.
func (s *Service) Update(ctx context.Context, id uuid.UUID, tenantID string, req UpdateListingRequest) (ListingResponse, error) {
	l, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ListingResponse{}, err
	}
	if l.TenantID != tenantID {
		return ListingResponse{}, ErrForbidden
	}
	if req.Name != nil {
		// Bug 114: name não pode ser vazio. Sem este gate admin pode
		// limpar via PATCH (Create rejeita name vazio).
		if *req.Name == "" {
			return ListingResponse{}, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		// Bug 140: name varchar(255) — gate length em Update.
		if len(*req.Name) > 255 {
			return ListingResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(*req.Name))
		}
		// Bug 183: strip HTML (XSS prevention).
		l.Name = sanitize.StripHTML(*req.Name)
	}
	if req.Description != nil {
		l.Description = *req.Description
	}
	if req.Category != nil {
		l.Category = *req.Category
	}
	updated, err := s.repo.Update(ctx, l)
	if err != nil {
		return ListingResponse{}, fmt.Errorf("listing: update: %w", err)
	}
	return ResponseFrom(updated), nil
}

// Delete soft-deletes a listing.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, tenantID string) error {
	l, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if l.TenantID != tenantID {
		return ErrForbidden
	}
	return s.repo.SoftDelete(ctx, id)
}

func toPage(listings []Listing, total int64, req pagination.PageRequest) pagination.Page[ListingResponse] {
	responses := make([]ListingResponse, len(listings))
	for i, l := range listings {
		responses[i] = ResponseFrom(l)
	}
	return pagination.NewPage(responses, total, req)
}

func toSlug(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
