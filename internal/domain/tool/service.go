package tool

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service holds business logic for tools.
type Service struct {
	repo ToolRepository
}

// NewService creates a new Service.
func NewService(repo ToolRepository) *Service {
	return &Service{repo: repo}
}

// List returns a paginated list of tools.
func (s *Service) List(ctx context.Context, req pagination.PageRequest, toolType string) (pagination.Page[Response], error) {
	tools, total, err := s.repo.List(ctx, req, toolType)
	if err != nil {
		return pagination.Page[Response]{}, err
	}
	content := make([]Response, len(tools))
	for i, t := range tools {
		content[i] = ResponseFrom(t)
	}
	return pagination.NewPage(content, total, req), nil
}

// Create creates a new tool.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Response, error) {
	if req.Name == "" {
		return Response{}, fmt.Errorf("tool: name is required")
	}
	if !IsValidToolType(req.Type) {
		return Response{}, fmt.Errorf("tool: unsupported type: %s", req.Type)
	}
	t := Tool{
		Name:        req.Name,
		Type:        req.Type,
		Config:      req.Config,
		Description: req.Description,
		Labels:      req.Labels,
	}
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(created), nil
}

// GetByID returns a tool by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Response, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(t), nil
}

// Update updates a tool.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	t, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(t), nil
}

// Delete deletes a tool.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// BindToSkill binds a tool to a skill.
func (s *Service) BindToSkill(ctx context.Context, skillID uuid.UUID, req BindRequest) (SkillToolResponse, error) {
	st, err := s.repo.BindToSkill(ctx, skillID, req)
	if err != nil {
		return SkillToolResponse{}, err
	}
	t, err := s.repo.GetByID(ctx, st.ToolID)
	if err != nil {
		return SkillToolResponse{}, err
	}
	return SkillToolResponse{
		ID:        st.ID,
		SkillID:   st.SkillID,
		Tool:      ResponseFrom(t),
		Priority:  st.Priority,
		IsActive:  st.IsActive,
		CreatedAt: st.CreatedAt,
	}, nil
}

// UnbindFromSkill removes a tool binding from a skill.
func (s *Service) UnbindFromSkill(ctx context.Context, skillID, toolID uuid.UUID) error {
	return s.repo.UnbindFromSkill(ctx, skillID, toolID)
}

// ListBySkill returns all tools bound to a skill.
func (s *Service) ListBySkill(ctx context.Context, skillID uuid.UUID) ([]SkillToolResponse, error) {
	bindings, tools, err := s.repo.ListBySkill(ctx, skillID)
	if err != nil {
		return nil, err
	}
	resp := make([]SkillToolResponse, len(bindings))
	for i, st := range bindings {
		resp[i] = SkillToolResponse{
			ID:        st.ID,
			SkillID:   st.SkillID,
			Tool:      ResponseFrom(tools[i]),
			Priority:  st.Priority,
			IsActive:  st.IsActive,
			CreatedAt: st.CreatedAt,
		}
	}
	return resp, nil
}
