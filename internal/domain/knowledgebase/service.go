package knowledgebase

import (
	"context"
	"fmt"
	"strings"

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
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: name is required")
	}
	if len(req.Name) > 255 {
		return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: name exceeds maximum length of 255 chars (got %d)", len(req.Name))
	}
	if req.SearchMode != "" {
		switch req.SearchMode {
		case "VECTOR", "KEYWORD", "HYBRID":
		default:
			return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: searchMode must be one of VECTOR, KEYWORD, HYBRID (got %q)", req.SearchMode)
		}
	}
	if req.EmbeddingModel != "" {
		switch req.EmbeddingModel {
		case "intfloat/multilingual-e5-large", "intfloat/multilingual-e5-base", "text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002":
		default:
			return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: embeddingModel %q not supported (allowed: intfloat/multilingual-e5-large, intfloat/multilingual-e5-base, text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002)", req.EmbeddingModel)
		}
	}

	if req.ContextWindow < 0 {
		return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: contextWindow must be >= 0 (got %d)", req.ContextWindow)
	}

	exists, err := s.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: check duplicate: %w", err)
	}
	if exists {
		return KnowledgeBaseResponse{}, ErrDuplicateName
	}

	kb := KnowledgeBase{
		Name:           req.Name,
		Description:    req.Description,
		Status:         StatusActive,
		EmbeddingModel: req.EmbeddingModel,
		SearchMode:     req.SearchMode,
		ContextWindow:  req.ContextWindow,
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
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return KnowledgeBaseResponse{}, fmt.Errorf("knowledgebase service: name cannot be empty")
		}
		// Bug 136: name varchar(255) — gate length em Update.
		if len(trimmed) > 255 {
			return KnowledgeBaseResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(trimmed))
		}
		existing.Name = trimmed
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.EmbeddingModel != nil {
		// Bug 149: embeddingModel aceita só lista pré-aprovada (Create
		// gateava; Update bypass deixava admin salvar "NOT-A-MODEL" e
		// embedding-job falhava silenciosamente em runtime).
		switch *req.EmbeddingModel {
		case "intfloat/multilingual-e5-large", "intfloat/multilingual-e5-base", "text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002":
		default:
			return KnowledgeBaseResponse{}, fmt.Errorf("%w: embeddingModel %q not supported (allowed: intfloat/multilingual-e5-large, intfloat/multilingual-e5-base, text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002)", ErrValidation, *req.EmbeddingModel)
		}
		existing.EmbeddingModel = *req.EmbeddingModel
	}
	if req.SearchMode != nil {
		// Bug 118: searchMode aceita só VECTOR, KEYWORD, HYBRID.
		// Create rejeita; Update precisa do mesmo gate. Sem isso,
		// admin podia salvar "INVALID" e o RAG retriever falharia
		// em runtime ao montar a query.
		switch *req.SearchMode {
		case "VECTOR", "KEYWORD", "HYBRID":
		default:
			return KnowledgeBaseResponse{}, fmt.Errorf("%w: searchMode must be one of VECTOR, KEYWORD, HYBRID (got %q)", ErrValidation, *req.SearchMode)
		}
		existing.SearchMode = *req.SearchMode
	}
	if req.ContextWindow != nil {
		// Bug 110: Update precisa do mesmo gate >= 0 que Create —
		// senão admin podia criar KB com contextWindow=2048 e depois
		// PATCH para -100, e o RAG retriever silenciosamente filtraria
		// nenhum chunk (negative > any positive count).
		if *req.ContextWindow < 0 {
			return KnowledgeBaseResponse{}, fmt.Errorf("%w: contextWindow must be >= 0 (got %d)", ErrValidation, *req.ContextWindow)
		}
		existing.ContextWindow = *req.ContextWindow
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
