package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

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
		SystemPrompt:   req.SystemPrompt,
		ModelConfig:    req.ModelConfig,
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
	if req.SystemPrompt != nil {
		a.SystemPrompt = req.SystemPrompt
	}
	if len(req.ModelConfig) > 0 {
		a.ModelConfig = req.ModelConfig
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
		SystemPrompt:   original.SystemPrompt,
		ModelConfig:    original.ModelConfig,
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

// VersionService defines business logic for AgentVersion.
type VersionService interface {
	CreateDraft(ctx context.Context, agentID uuid.UUID, req CreateAgentVersionRequest) (AgentVersionResponse, error)
	UpdateDraft(ctx context.Context, versionID uuid.UUID, req UpdateAgentVersionRequest) (AgentVersionResponse, error)
	Publish(ctx context.Context, versionID uuid.UUID) (AgentVersionResponse, error)
	GetDraft(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error)
	GetLatestPublished(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error)
	ListVersions(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[AgentVersionResponse], error)
}

type versionService struct {
	repo    Repository
	verRepo VersionRepository
}

// NewVersionService creates a new VersionService.
func NewVersionService(repo Repository, verRepo VersionRepository) VersionService {
	return &versionService{repo: repo, verRepo: verRepo}
}

func (s *versionService) CreateDraft(ctx context.Context, agentID uuid.UUID, req CreateAgentVersionRequest) (AgentVersionResponse, error) {
	// Ensure the agent exists.
	if _, err := s.repo.FindByID(ctx, agentID); err != nil {
		return AgentVersionResponse{}, err
	}
	// Ensure no existing draft.
	if _, err := s.verRepo.FindDraft(ctx, agentID); err == nil {
		return AgentVersionResponse{}, ErrDraftAlreadyExists
	}
	num, err := s.verRepo.NextVersionNumber(ctx, agentID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	v := AgentVersion{
		ID:             uuid.New(),
		AgentID:        agentID,
		VersionNumber:  num,
		Status:         VersionStatusDraft,
		Description:    req.Description,
		DefinitionJSON: req.DefinitionJSON,
		ConfigJSON:     req.ConfigJSON,
	}
	created, err := s.verRepo.Create(ctx, v)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	return VersionResponseFrom(created), nil
}

func (s *versionService) UpdateDraft(ctx context.Context, versionID uuid.UUID, req UpdateAgentVersionRequest) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindByID(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	if v.Status != VersionStatusDraft {
		return AgentVersionResponse{}, ErrVersionImmutable
	}
	if req.Description != nil {
		v.Description = *req.Description
	}
	if len(req.DefinitionJSON) > 0 {
		v.DefinitionJSON = req.DefinitionJSON
	}
	if len(req.ConfigJSON) > 0 {
		v.ConfigJSON = req.ConfigJSON
	}
	updated, err := s.verRepo.Update(ctx, v)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	return VersionResponseFrom(updated), nil
}

func (s *versionService) Publish(ctx context.Context, versionID uuid.UUID) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindByID(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	if v.Status != VersionStatusDraft {
		return AgentVersionResponse{}, ErrVersionImmutable
	}
	published, err := s.verRepo.Publish(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	return VersionResponseFrom(published), nil
}

func (s *versionService) GetDraft(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindDraft(ctx, agentID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	return VersionResponseFrom(v), nil
}

func (s *versionService) GetLatestPublished(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindLatestPublished(ctx, agentID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	return VersionResponseFrom(v), nil
}

func (s *versionService) ListVersions(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[AgentVersionResponse], error) {
	versions, total, err := s.verRepo.FindByAgentID(ctx, agentID, req)
	if err != nil {
		return pagination.Page[AgentVersionResponse]{}, err
	}
	responses := make([]AgentVersionResponse, len(versions))
	for i, v := range versions {
		responses[i] = VersionResponseFrom(v)
	}
	return pagination.NewPage(responses, total, req), nil
}
