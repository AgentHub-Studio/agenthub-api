package llmpreset

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service defines business logic operations for LLMPreset.
type Service interface {
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[LLMPresetResponse], error)
	Get(ctx context.Context, id uuid.UUID) (LLMPresetResponse, error)
	Create(ctx context.Context, req CreateLLMPresetRequest) (LLMPresetResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateLLMPresetRequest) (LLMPresetResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetDefault(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repo Repository
}

// NewService creates a new LLMPreset Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[LLMPresetResponse], error) {
	presets, total, err := s.repo.FindAll(ctx, req)
	if err != nil {
		return pagination.Page[LLMPresetResponse]{}, err
	}
	responses := make([]LLMPresetResponse, len(presets))
	for i, p := range presets {
		responses[i] = ResponseFrom(p)
	}
	return pagination.NewPage(responses, total, req), nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (LLMPresetResponse, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	return ResponseFrom(p), nil
}

func (s *service) Create(ctx context.Context, req CreateLLMPresetRequest) (LLMPresetResponse, error) {
	if req.Name == "" {
		return LLMPresetResponse{}, fmt.Errorf("name is required")
	}
	if req.Provider == "" {
		return LLMPresetResponse{}, fmt.Errorf("provider is required")
	}
	if req.Model == "" {
		return LLMPresetResponse{}, fmt.Errorf("model is required")
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
		ID:          uuid.New(),
		Name:        req.Name,
		Provider:    req.Provider,
		Model:       req.Model,
		BaseURL:     req.BaseURL,
		APIKeyEnv:   req.APIKeyEnv,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		IsDefault:   req.IsDefault,
	}
	created, err := s.repo.Create(ctx, p)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	return ResponseFrom(created), nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateLLMPresetRequest) (LLMPresetResponse, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Provider != nil {
		p.Provider = *req.Provider
	}
	if req.Model != nil {
		p.Model = *req.Model
	}
	if req.BaseURL != nil {
		p.BaseURL = *req.BaseURL
	}
	if req.APIKeyEnv != nil {
		p.APIKeyEnv = *req.APIKeyEnv
	}
	if req.MaxTokens != nil {
		p.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		p.Temperature = *req.Temperature
	}
	updated, err := s.repo.Update(ctx, p)
	if err != nil {
		return LLMPresetResponse{}, err
	}
	return ResponseFrom(updated), nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) SetDefault(ctx context.Context, id uuid.UUID) error {
	return s.repo.SetDefault(ctx, id)
}
