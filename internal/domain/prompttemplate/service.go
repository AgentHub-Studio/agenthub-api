package prompttemplate

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
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
	if req.Name == "" {
		return Response{}, fmt.Errorf("prompt template: name is required")
	}
	if req.Slug == "" {
		return Response{}, fmt.Errorf("prompt template: slug is required")
	}
	if !slugPattern.MatchString(req.Slug) {
		return Response{}, fmt.Errorf("%w: slug must match [a-z0-9][a-z0-9-]* (got %q)", ErrValidation, req.Slug)
	}
	if req.Content == "" {
		return Response{}, fmt.Errorf("prompt template: content is required")
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
		existing.Name = *req.Name
	}
	if req.Slug != nil {
		existing.Slug = *req.Slug
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Content != nil {
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
