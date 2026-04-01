package knowledgebase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service provides business logic for KnowledgeBase operations.
type Service struct {
	repo Repository
}

// NewService creates a new Service backed by the given Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns a paginated list of knowledge bases.
func (s *Service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[KnowledgeBaseResponse], error) {
	items, total, err := s.repo.List(ctx, req)
	if err != nil {
		return pagination.Page[KnowledgeBaseResponse]{}, fmt.Errorf("knowledgebase service: list: %w", err)
	}

	responses := make([]KnowledgeBaseResponse, len(items))
	for i, item := range items {
		responses[i] = ResponseFrom(item)
	}

	return pagination.NewPage(responses, total, req), nil
}

// GetByID returns a single knowledge base by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (KnowledgeBaseResponse, error) {
	kb, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return KnowledgeBaseResponse{}, err
	}
	return ResponseFrom(kb), nil
}

// Create creates a new knowledge base.
func (s *Service) Create(ctx context.Context, req CreateRequest) (KnowledgeBaseResponse, error) {
	if req.Name == "" {
		return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: name is required")
	}

	kb := KnowledgeBase{
		Name:        req.Name,
		Description: req.Description,
		Status:      StatusActive,
	}

	created, err := s.repo.Create(ctx, kb)
	if err != nil {
		return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: create: %w", err)
	}

	return ResponseFrom(created), nil
}

// Update updates an existing knowledge base.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (KnowledgeBaseResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return KnowledgeBaseResponse{}, err
	}

	if req.Name != nil {
		if *req.Name == "" {
			return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: name cannot be empty")
		}
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: update: %w", err)
	}

	return ResponseFrom(updated), nil
}

// Delete removes a knowledge base by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("knowledgebase service: delete: %w", err)
	}
	return nil
}

// Pause sets the knowledge base status to PAUSED.
func (s *Service) Pause(ctx context.Context, id uuid.UUID) (KnowledgeBaseResponse, error) {
	kb, err := s.repo.UpdateStatus(ctx, id, StatusPaused)
	if err != nil {
		return KnowledgeBaseResponse{}, err
	}
	return ResponseFrom(kb), nil
}

// Activate sets the knowledge base status to ACTIVE.
func (s *Service) Activate(ctx context.Context, id uuid.UUID) (KnowledgeBaseResponse, error) {
	kb, err := s.repo.UpdateStatus(ctx, id, StatusActive)
	if err != nil {
		return KnowledgeBaseResponse{}, err
	}
	return ResponseFrom(kb), nil
}
