package llmpreset

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service defines business logic operations for LLMPreset.
type Service interface {
	List(ctx context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[LLMPresetResponse], error)
	ListByProvider(ctx context.Context, tenantID, provider string, req pagination.PageRequest) (pagination.Page[LLMPresetResponse], error)
	Get(ctx context.Context, tenantID string, id uuid.UUID) (LLMPresetResponse, error)
	Create(ctx context.Context, tenantID string, req CreateLLMPresetRequest) (LLMPresetResponse, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req UpdateLLMPresetRequest) (LLMPresetResponse, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
	SetDefault(ctx context.Context, tenantID string, id uuid.UUID) error
}

type service struct {
	repo Repository
}

// NewService creates a new LLMPreset Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[LLMPresetResponse], error) {
	presets, total, err := s.repo.FindAll(ctx, tenantID, req)
	if err != nil {
		return pagination.Page[LLMPresetResponse]{}, err
	}
	responses := make([]LLMPresetResponse, len(presets))
	for i, p := range presets {
		responses[i] = ResponseFrom(p)
	}
	return pagination.NewPage(responses, total, req), nil
}

func (s *service) ListByProvider(ctx context.Context, tenantID, provider string, req pagination.PageRequest) (pagination.Page[LLMPresetResponse], error) {
	if provider == "" {
		return pagination.Page[LLMPresetResponse]{}, fmt.Errorf("llmpreset: provider is required")
	}
	presets, total, err := s.repo.FindByProvider(ctx, tenantID, provider, req)
	if err != nil {
		return pagination.Page[LLMPresetResponse]{}, err
	}
	responses := make([]LLMPresetResponse, len(presets))
	for i, p := range presets {
		responses[i] = ResponseFrom(p)
	}
	return pagination.NewPage(responses, total, req), nil
}

func (s *service) Get(ctx context.Context, tenantID string, id uuid.UUID) (LLMPresetResponse, error) {
	p, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	return ResponseFrom(p), nil
}

func (s *service) Create(ctx context.Context, tenantID string, req CreateLLMPresetRequest) (LLMPresetResponse, error) {
	if req.Name == "" {
		return LLMPresetResponse{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	// Bug 130: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return LLMPresetResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	if req.Provider == "" {
		return LLMPresetResponse{}, fmt.Errorf("%w: provider is required", ErrValidation)
	}
	switch req.Provider {
	case "openai", "anthropic", "ollama", "openrouter", "google", "azure", "bedrock", "vertex", "groq", "mistral":
	default:
		return LLMPresetResponse{}, fmt.Errorf("%w: provider must be one of openai, anthropic, ollama, openrouter, google, azure, bedrock, vertex, groq, mistral (got %q)", ErrValidation, req.Provider)
	}
	if req.Model == "" {
		return LLMPresetResponse{}, fmt.Errorf("%w: model is required", ErrValidation)
	}
	if req.Temperature < 0 || req.Temperature > 2 {
		return LLMPresetResponse{}, fmt.Errorf("%w: temperature must be between 0 and 2 (got %v)", ErrValidation, req.Temperature)
	}
	if req.MaxTokens < 0 || req.MaxTokens > 2_000_000 {
		// Bug 153: cap em 2M (espelha bug 152 em agent modelConfig).
		// LLMs reais aceitam ~128k-1M no max; valor maior é payload
		// malformado e gera custo/timeout no provider.
		return LLMPresetResponse{}, fmt.Errorf("%w: maxTokens must be between 0 and 2000000 (got %d)", ErrValidation, req.MaxTokens)
	}
	if req.ContextWindow < 0 || req.ContextWindow > 2_000_000 {
		return LLMPresetResponse{}, fmt.Errorf("%w: contextWindow must be between 0 and 2000000 (got %d)", ErrValidation, req.ContextWindow)
	}
	exists, err := s.repo.ExistsByName(ctx, tenantID, req.Name)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	if exists {
		return LLMPresetResponse{}, ErrDuplicateName
	}
	// Bug 160: cap description em 32KB.
	if len(req.Description) > 32000 {
		return LLMPresetResponse{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	p := LLMPreset{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Name:          req.Name,
		Description:   req.Description,
		Provider:      req.Provider,
		Model:         req.Model,
		MaxTokens:     maxTokens,
		ContextWindow: req.ContextWindow,
		Temperature:   temperature,
		ConfigJSON:    req.ConfigJSON,
		IsDefault:     req.IsDefault,
	}
	created, err := s.repo.Create(ctx, p)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	return ResponseFrom(created), nil
}

func (s *service) Update(ctx context.Context, tenantID string, id uuid.UUID, req UpdateLLMPresetRequest) (LLMPresetResponse, error) {
	p, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	if req.Name != nil {
		// Bug 137: name varchar(255) — gate length em Update.
		if len(*req.Name) > 255 {
			return LLMPresetResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(*req.Name))
		}
		p.Name = *req.Name
	}
	if req.Description != nil {
		// Bug 176: cap em Update (cross-cutting com Create — bug 160).
		if len(*req.Description) > 32000 {
			return LLMPresetResponse{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(*req.Description))
		}
		p.Description = *req.Description
	}
	if req.Provider != nil {
		// Bug 111: Update precisa do mesmo enum gate que Create.
		switch *req.Provider {
		case "openai", "anthropic", "ollama", "openrouter", "google", "azure", "bedrock", "vertex", "groq", "mistral":
		default:
			return LLMPresetResponse{}, fmt.Errorf("%w: provider must be one of openai, anthropic, ollama, openrouter, google, azure, bedrock, vertex, groq, mistral (got %q)", ErrValidation, *req.Provider)
		}
		p.Provider = *req.Provider
	}
	if req.Model != nil {
		p.Model = *req.Model
	}
	if req.MaxTokens != nil {
		// Bug 111+153: maxTokens [0, 2M] (Create rejeita negativos e
		// valores absurdos > 2M).
		if *req.MaxTokens < 0 || *req.MaxTokens > 2_000_000 {
			return LLMPresetResponse{}, fmt.Errorf("%w: maxTokens must be between 0 and 2000000 (got %d)", ErrValidation, *req.MaxTokens)
		}
		p.MaxTokens = *req.MaxTokens
	}
	if req.ContextWindow != nil {
		// Bug 111+153: contextWindow [0, 2M] (mesmo gate que Create).
		if *req.ContextWindow < 0 || *req.ContextWindow > 2_000_000 {
			return LLMPresetResponse{}, fmt.Errorf("%w: contextWindow must be between 0 and 2000000 (got %d)", ErrValidation, *req.ContextWindow)
		}
		p.ContextWindow = *req.ContextWindow
	}
	if req.Temperature != nil {
		// Bug 111: temperature 0-2 (mesmo gate que Create). Sem isso, admin
		// pode setar 99.9 via PATCH e o LLM client falhará em runtime
		// com erro opaco.
		if *req.Temperature < 0 || *req.Temperature > 2 {
			return LLMPresetResponse{}, fmt.Errorf("%w: temperature must be between 0 and 2 (got %v)", ErrValidation, *req.Temperature)
		}
		p.Temperature = *req.Temperature
	}
	if req.ConfigJSON != nil {
		p.ConfigJSON = req.ConfigJSON
	}
	updated, err := s.repo.Update(ctx, p)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	return ResponseFrom(updated), nil
}

func (s *service) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}

func (s *service) SetDefault(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.SetDefault(ctx, tenantID, id)
}
