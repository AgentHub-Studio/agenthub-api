package prompttemplate

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Service encapsulates prompt template business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns paginated templates, optionally filtered by category.
func (s *Service) List(ctx context.Context, category string, req pagination.PageRequest) (pagination.Page[Response], error) {
	templates, total, err := s.repo.ListAll(ctx, category, req)
	if err != nil {
		return pagination.Page[Response]{}, err
	}
	responses := make([]Response, len(templates))
	for i, t := range templates {
		responses[i] = ResponseFrom(t)
	}
	return pagination.NewPage(responses, total, req), nil
}

// ListByAgent returns templates for a specific agent (including global ones).
func (s *Service) ListByAgent(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[Response], error) {
	templates, total, err := s.repo.ListByAgent(ctx, agentID, req)
	if err != nil {
		return pagination.Page[Response]{}, err
	}
	responses := make([]Response, len(templates))
	for i, t := range templates {
		responses[i] = ResponseFrom(t)
	}
	return pagination.NewPage(responses, total, req), nil
}

// Get returns a single template by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Response, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(t), nil
}

// Create creates a new prompt template.
func (s *Service) Create(ctx context.Context, agentID *uuid.UUID, req CreateRequest) (Response, error) {
	// Bug 181: strip HTML do name (XSS prevention).
	req.Name = sanitize.StripHTML(req.Name)
	if req.Name == "" {
		return Response{}, fmt.Errorf("prompt template: name is required")
	}
	// Bug 131: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return Response{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	if req.Slug == "" {
		return Response{}, fmt.Errorf("prompt template: slug is required")
	}
	if !slugPattern.MatchString(req.Slug) {
		return Response{}, fmt.Errorf("%w: slug must match [a-z0-9][a-z0-9-]* (got %q)", ErrValidation, req.Slug)
	}
	// Bug 133: slug varchar(255) — gate length antes do INSERT.
	if len(req.Slug) > 255 {
		return Response{}, fmt.Errorf("%w: slug exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Slug))
	}
	if req.Content == "" {
		return Response{}, fmt.Errorf("prompt template: content is required")
	}
	// Bug 158: cap content em 32KB. Sem isso, prompt-template aceita
	// 200KB+ silenciosamente (DoS storage + perf hit).
	if len(req.Content) > 32000 {
		return Response{}, fmt.Errorf("%w: content exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Content))
	}
	// Bug 160: cap description em 32KB.
	if len(req.Description) > 32000 {
		return Response{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	}
	// Bug 180: strip HTML do description (XSS prevention).
	req.Description = sanitize.StripHTML(req.Description)

	exists, err := s.repo.ExistsBySlug(ctx, agentID, req.Slug)
	if err != nil {
		return Response{}, fmt.Errorf("prompt template: check duplicate: %w", err)
	}
	if exists {
		return Response{}, ErrDuplicateSlug
	}

	category := CategoryCustom
	if req.Category != "" {
		category = Category(req.Category)
	}

	t := PromptTemplate{
		AgentID:       agentID,
		Name:          req.Name,
		Slug:          req.Slug,
		Description:   req.Description,
		Content:       req.Content,
		Category:      category,
		ModelOverride: req.ModelOverride,
		AllowedTools:  req.AllowedTools,
	}

	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(created), nil
}

// Update modifies an existing prompt template.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Response{}, err
	}

	if req.Name != nil {
		// Bug 113: name não pode ser vazio. Sem este gate admin podia
		// limpar via PATCH (Create rejeita name vazio).
		if *req.Name == "" {
			return Response{}, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		// Bug 137: name varchar(255) — gate length em Update.
		if len(*req.Name) > 255 {
			return Response{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(*req.Name))
		}
		// Bug 181: strip HTML (XSS prevention).
		existing.Name = sanitize.StripHTML(*req.Name)
	}
	if req.Slug != nil {
		// Bug 120: Update precisa do mesmo gate que Create — pattern
		// `[a-z0-9][a-z0-9-]*` + não vazio. Sem isso admin podia salvar
		// slug="" ou slug="INVALID!" via PATCH e quebrar lookups por slug.
		if *req.Slug == "" {
			return Response{}, fmt.Errorf("%w: slug cannot be empty", ErrValidation)
		}
		if !slugPattern.MatchString(*req.Slug) {
			return Response{}, fmt.Errorf("%w: slug must match [a-z0-9][a-z0-9-]* (got %q)", ErrValidation, *req.Slug)
		}
		// Bug 138: slug varchar(255) — gate length em Update.
		if len(*req.Slug) > 255 {
			return Response{}, fmt.Errorf("%w: slug exceeds maximum length of 255 chars (got %d)", ErrValidation, len(*req.Slug))
		}
		existing.Slug = *req.Slug
	}
	if req.Description != nil {
		// Bug 175: cap em Update (cross-cutting com Create — bug 159).
		if len(*req.Description) > 32000 {
			return Response{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(*req.Description))
		}
		// Bug 180: strip HTML (XSS prevention).
		existing.Description = sanitize.StripHTML(*req.Description)
	}
	if req.Content != nil {
		// Bug 113: content="" deixa o template inutilizável (renderiza
		// vazio). Create rejeita; Update precisa do mesmo gate.
		if *req.Content == "" {
			return Response{}, fmt.Errorf("%w: content cannot be empty", ErrValidation)
		}
		// Bug 175: cap em Update (cross-cutting com Create — bug 158).
		if len(*req.Content) > 32000 {
			return Response{}, fmt.Errorf("%w: content exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(*req.Content))
		}
		existing.Content = *req.Content
	}
	if req.Category != nil {
		existing.Category = Category(*req.Category)
	}
	if req.ModelOverride != nil {
		existing.ModelOverride = req.ModelOverride
	}
	if len(req.AllowedTools) > 0 {
		existing.AllowedTools = req.AllowedTools
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(updated), nil
}

// Delete removes a prompt template by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
