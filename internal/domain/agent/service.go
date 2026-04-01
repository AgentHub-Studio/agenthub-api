package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

var slugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Service defines business logic operations for Agent.
type Service interface {
	List(ctx context.Context, status AgentStatus, req pagination.PageRequest) (pagination.Page[AgentResponse], error)
	Get(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Create(ctx context.Context, req CreateAgentRequest) (AgentResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateAgentRequest) (AgentResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Publish(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Archive(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Clone(ctx context.Context, id uuid.UUID, req CloneAgentRequest) (AgentResponse, error)
}

type service struct {
	repo Repository
}

// NewService creates a new agent Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, status AgentStatus, req pagination.PageRequest) (pagination.Page[AgentResponse], error) {
	agents, total, err := s.repo.FindAll(ctx, status, req)
	if err != nil {
		return pagination.Page[AgentResponse]{}, err
	}
	responses := make([]AgentResponse, len(agents))
	for i, a := range agents {
		responses[i] = ResponseFrom(a)
	}
	return pagination.NewPage(responses, total, req), nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	return ResponseFrom(a), nil
}

func (s *service) Create(ctx context.Context, req CreateAgentRequest) (AgentResponse, error) {
	if req.Name == "" {
		return AgentResponse{}, fmt.Errorf("name is required")
	}
	slug := req.Slug
	if slug == "" {
		slug = toSlug(req.Name)
	}
	config := req.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	a := Agent{
		ID:             uuid.New(),
		Name:           req.Name,
		Slug:           slug,
		Description:    req.Description,
		Status:         StatusDraft,
		CurrentVersion: 1,
		PipelineID:     req.PipelineID,
		Config:         config,
	}
	created, err := s.repo.Create(ctx, a)
	if err != nil {
		return AgentResponse{}, err
	}
	return ResponseFrom(created), nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateAgentRequest) (AgentResponse, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Slug != nil {
		a.Slug = *req.Slug
	}
	if req.Description != nil {
		a.Description = *req.Description
	}
	if req.PipelineID != nil {
		a.PipelineID = req.PipelineID
	}
	if len(req.Config) > 0 {
		a.Config = req.Config
	}
	updated, err := s.repo.Update(ctx, a)
	if err != nil {
		return AgentResponse{}, err
	}
	return ResponseFrom(updated), nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) Publish(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	a, err := s.repo.UpdateStatus(ctx, id, StatusPublished)
	if err != nil {
		return AgentResponse{}, err
	}
	return ResponseFrom(a), nil
}

func (s *service) Archive(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	a, err := s.repo.UpdateStatus(ctx, id, StatusArchived)
	if err != nil {
		return AgentResponse{}, err
	}
	return ResponseFrom(a), nil
}

func (s *service) Clone(ctx context.Context, id uuid.UUID, req CloneAgentRequest) (AgentResponse, error) {
	original, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	name := req.Name
	if name == "" {
		name = original.Name + " (copy)"
	}
	clone := Agent{
		ID:             uuid.New(),
		Name:           name,
		Slug:           toSlug(name),
		Description:    original.Description,
		Status:         StatusDraft,
		CurrentVersion: 1,
		PipelineID:     original.PipelineID,
		Config:         original.Config,
	}
	created, err := s.repo.Create(ctx, clone)
	if err != nil {
		return AgentResponse{}, err
	}
	return ResponseFrom(created), nil
}

// toSlug converts a name to a kebab-case slug.
func toSlug(name string) string {
	s := strings.ToLower(name)
	// Replace non-alphanumeric characters with hyphens.
	var b strings.Builder
	prevHyphen := true
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	if result == "" {
		return "agent-" + uuid.New().String()[:8]
	}
	return result
}
