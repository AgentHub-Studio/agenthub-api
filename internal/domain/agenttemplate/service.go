package agenttemplate

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// agentCreator is a narrow interface for creating agents from a template definition.
// Implemented by *agent.service — declared here to avoid a circular dependency.
type agentCreator interface {
	Create(ctx context.Context, req agent.CreateAgentRequest) (agent.AgentResponse, error)
}

// Service holds business logic for agent templates.
type Service struct {
	repo        Repository
	agentCreator agentCreator
}

// NewService creates a new Service backed by the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// WithAgentCreator attaches the agent creator used during instantiation.
func (s *Service) WithAgentCreator(c agentCreator) *Service {
	s.agentCreator = c
	return s
}

// ListAll returns all templates, ordered by builtin-first then name.
// If category is non-empty, results are filtered to that category.
func (s *Service) ListAll(ctx context.Context, category string) ([]TemplateResponse, error) {
	var (
		templates []AgentTemplate
		err       error
	)
	if category != "" {
		templates, err = s.repo.ListByCategory(ctx, category)
	} else {
		templates, err = s.repo.ListAll(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]TemplateResponse, len(templates))
	for i, t := range templates {
		out[i] = ResponseFrom(t)
	}
	return out, nil
}

// GetBySlug returns the template with the given slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (TemplateResponse, error) {
	t, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return TemplateResponse{}, err
	}
	return ResponseFrom(t), nil
}

// Create stores a new tenant-owned (non-builtin) template.
func (s *Service) Create(ctx context.Context, req CreateTemplateRequest) (TemplateResponse, error) {
	if req.Name == "" {
		return TemplateResponse{}, fmt.Errorf("name is required")
	}
	// Bug 131: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return TemplateResponse{}, fmt.Errorf("name exceeds maximum length of 255 chars (got %d)", len(req.Name))
	}
	slug := req.Slug
	if slug == "" {
		slug = toSlug(req.Name)
	} else if !slugPattern.MatchString(slug) {
		return TemplateResponse{}, fmt.Errorf("agent template: slug must match [a-z0-9][a-z0-9-]* (got %q)", slug)
	}
	// Bug 133: slug varchar(255) — gate length antes do INSERT.
	if len(slug) > 255 {
		return TemplateResponse{}, fmt.Errorf("agent template: slug exceeds maximum length of 255 chars (got %d)", len(slug))
	}
	t := AgentTemplate{
		Name:           req.Name,
		Slug:           slug,
		Description:    req.Description,
		Category:       req.Category,
		IsBuiltin:      false,
		DefinitionJSON: req.Definition,
	}
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return TemplateResponse{}, err
	}
	return ResponseFrom(created), nil
}

// Instantiate creates an Agent from a template definition.
// The agent is created in DRAFT status with system prompt and model config from the template.
// Skill bindings (by slug) are stored in the template definition as documentation;
// skill IDs must be resolved and bound separately by the caller if needed.
func (s *Service) Instantiate(ctx context.Context, slug string, req InstantiateRequest) (InstantiateResponse, error) {
	if s.agentCreator == nil {
		return InstantiateResponse{}, fmt.Errorf("agent creator not configured")
	}
	t, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return InstantiateResponse{}, err
	}
	def := t.ParseDefinition()

	name := req.Name
	if name == "" {
		name = t.Name
	}
	description := req.Description
	if description == "" {
		description = t.Description
	}

	sp := def.SystemPrompt
	createReq := agent.CreateAgentRequest{
		Name:            name,
		Description:     description,
		SystemPrompt:    &sp,
		ModelConfig:     def.ModelConfig,
		PermissionRules: def.PermissionRules,
	}

	created, err := s.agentCreator.Create(ctx, createReq)
	if err != nil {
		return InstantiateResponse{}, fmt.Errorf("instantiate template %q: %w", slug, err)
	}

	return InstantiateResponse{
		AgentID: created.ID,
		Name:    created.Name,
		Slug:    created.Slug,
	}, nil
}

// toSlug generates a URL-safe slug from s.
func toSlug(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return '-'
	}, s)
	// Collapse consecutive dashes.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

