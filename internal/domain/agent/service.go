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
	List(ctx context.Context, status AgentStatus, q string, req pagination.PageRequest) (pagination.Page[AgentResponse], error)
	Get(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Create(ctx context.Context, req CreateAgentRequest) (AgentResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateAgentRequest) (AgentResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Publish(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Archive(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Clone(ctx context.Context, id uuid.UUID, req CloneAgentRequest) (AgentResponse, error)
}

type service struct {
	repo        Repository
	bindingRepo BindingRepository
}

// NewService creates a new agent Service.
func NewService(repo Repository, bindingRepo BindingRepository) Service {
	return &service{repo: repo, bindingRepo: bindingRepo}
}

func (s *service) List(ctx context.Context, status AgentStatus, q string, req pagination.PageRequest) (pagination.Page[AgentResponse], error) {
	agents, total, err := s.repo.FindAll(ctx, status, q, req)
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
	resp := ResponseFrom(a)
	if skillIDs, err := s.bindingRepo.ListSkillIDs(ctx, id); err == nil {
		resp.SkillIDs = skillIDs
	}
	return resp, nil
}

func (s *service) Create(ctx context.Context, req CreateAgentRequest) (AgentResponse, error) {
	if req.Name == "" {
		return AgentResponse{}, fmt.Errorf("name is required")
	}
	// P-C249-2: reject modelConfig nested inside the config field. Clients must
	// send modelConfig at the root level of the request body.
	if hasNestedModelConfig(req.Config) {
		return AgentResponse{}, fmt.Errorf("%w: modelConfig must be at the root of the request body, not inside config", ErrInvalidRequest)
	}
	// P-C97-1: reject invalid modelConfig at creation time so the agent is never
	// stored in a broken state (e.g. maxIterations=-5 makes the loop exit immediately).
	if err := validateModelConfig(req.ModelConfig); err != nil {
		return AgentResponse{}, fmt.Errorf("%w: %s", ErrInvalidModelConfig, err)
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
		ID:               uuid.New(),
		Name:             req.Name,
		Slug:             slug,
		Description:      req.Description,
		Status:           StatusDraft,
		CurrentVersion:   1,
		SystemPrompt:     req.SystemPrompt,
		ModelConfig:      req.ModelConfig,
		PermissionRules:  req.PermissionRules,
		Config:           config,
		EnableManagement: req.EnableManagement,
	}
	created, err := s.repo.Create(ctx, a)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(created)
	// Link skills provided in the creation request (P-C60-1 fix).
	if len(req.SkillIDs) > 0 {
		if syncErr := s.bindingRepo.SyncSkills(ctx, created.ID, req.SkillIDs); syncErr != nil {
			// Roll back by deleting the just-created agent so the caller sees a clean failure.
			_ = s.repo.Delete(ctx, created.ID)
			return AgentResponse{}, fmt.Errorf("%w: %w", ErrInvalidSkillIDs, syncErr)
		}
		resp.SkillIDs = req.SkillIDs
	}
	return resp, nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateAgentRequest) (AgentResponse, error) {
	// P-C249-2: same guard as Create — reject nested modelConfig.
	if hasNestedModelConfig(req.Config) {
		return AgentResponse{}, fmt.Errorf("%w: modelConfig must be at the root of the request body, not inside config", ErrInvalidRequest)
	}
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
		if err := validateModelConfig(req.ModelConfig); err != nil {
			return AgentResponse{}, fmt.Errorf("%w: %s", ErrInvalidModelConfig, err)
		}
		a.ModelConfig = req.ModelConfig
	}
	if len(req.PermissionRules) > 0 {
		a.PermissionRules = req.PermissionRules
	}
	if len(req.Config) > 0 {
		a.Config = req.Config
	}
	if req.EnableManagement != nil {
		a.EnableManagement = *req.EnableManagement
	}
	updated, err := s.repo.Update(ctx, a)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(updated)
	if req.SkillIDs != nil {
		if syncErr := s.bindingRepo.SyncSkills(ctx, id, req.SkillIDs); syncErr != nil {
			return AgentResponse{}, syncErr
		}
		resp.SkillIDs = req.SkillIDs
	} else if skillIDs, err := s.bindingRepo.ListSkillIDs(ctx, id); err == nil {
		resp.SkillIDs = skillIDs
	}
	return resp, nil
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
		ID:              uuid.New(),
		Name:            name,
		Slug:            toSlug(name),
		Description:     original.Description,
		Status:          StatusDraft,
		CurrentVersion:  1,
		SystemPrompt:    original.SystemPrompt,
		ModelConfig:     original.ModelConfig,
		PermissionRules: original.PermissionRules,
		Config:          original.Config,
	}
	created, err := s.repo.Create(ctx, clone)
	if err != nil {
		return AgentResponse{}, err
	}
	return ResponseFrom(created), nil
}

// SupportedProviders lists the LLM providers recognised by the platform runner.
// P-C268-1, P-C327-3: provider is validated at agent creation/update time.
var SupportedProviders = []string{
	"openai",
	"openrouter",
	"anthropic",
	"ollama",
	"azure-openai",
	"google",
}

// ErrUnsupportedProvider is returned when modelConfig.provider is not in SupportedProviders.
var ErrUnsupportedProvider = fmt.Errorf("unsupported LLM provider")

// validateModelConfig checks that modelConfig contains valid JSON and that
// numeric fields are within safe ranges. Returns nil when raw is empty.
// P-C97-1: prevents agents with broken model_config from being stored.
// P-C294-2: validates provider/model consistency — if one is set, both must be.
// P-C268-1: validates provider against supported enum.
func validateModelConfig(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	// Must be a valid JSON object (not a string, array, etc.)
	var mc struct {
		Provider      string   `json:"provider"`
		Model         string   `json:"model"`
		MaxIterations *int     `json:"maxIterations"`
		MaxTokens     *int     `json:"maxTokens"`
		ContextWindow *int     `json:"contextWindow"`
		MaxDepth      *int     `json:"maxDepth"`
		Temperature   *float64 `json:"temperature"`
	}
	if err := json.Unmarshal(raw, &mc); err != nil {
		return fmt.Errorf("must be a valid JSON object")
	}
	// P-C268-1: validate provider against supported enum.
	if mc.Provider != "" {
		supported := false
		for _, p := range SupportedProviders {
			if mc.Provider == p {
				supported = true
				break
			}
		}
		if !supported {
			return fmt.Errorf("%w: %q — supported providers: %s",
				ErrUnsupportedProvider, mc.Provider, strings.Join(SupportedProviders, ", "))
		}
	}
	// P-C294-2: provider and model are a pair — both or neither.
	if mc.Provider != "" && mc.Model == "" {
		return fmt.Errorf("model is required when provider is specified")
	}
	if mc.Model != "" && mc.Provider == "" {
		return fmt.Errorf("provider is required when model is specified")
	}
	if mc.MaxIterations != nil && *mc.MaxIterations <= 0 {
		return fmt.Errorf("maxIterations must be a positive integer (got %d)", *mc.MaxIterations)
	}
	if mc.MaxTokens != nil && *mc.MaxTokens <= 0 {
		return fmt.Errorf("maxTokens must be a positive integer (got %d)", *mc.MaxTokens)
	}
	if mc.ContextWindow != nil && *mc.ContextWindow <= 0 {
		return fmt.Errorf("contextWindow must be a positive integer (got %d)", *mc.ContextWindow)
	}
	if mc.MaxDepth != nil && *mc.MaxDepth < 0 {
		return fmt.Errorf("maxDepth must be a non-negative integer (got %d)", *mc.MaxDepth)
	}
	if mc.Temperature != nil && (*mc.Temperature < 0 || *mc.Temperature > 2.0) {
		return fmt.Errorf("temperature must be between 0.0 and 2.0 (got %g)", *mc.Temperature)
	}
	return nil
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

// hasNestedModelConfig returns true when the given config JSON blob contains a
// top-level "modelConfig" key. P-C249-2: clients that accidentally nest modelConfig
// inside the config field would silently produce non-functional agents; reject early.
func hasNestedModelConfig(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false
	}
	_, ok := cfg["modelConfig"]
	return ok
}
