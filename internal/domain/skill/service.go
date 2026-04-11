package skill

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ErrSkillBoundToAgents is returned when a skill cannot be deleted because one or
// more agents still reference it. P-C185-1: management executor must not bypass this check.
var ErrSkillBoundToAgents = errors.New("skill: cannot delete — skill is bound to one or more agents")

// ErrSkillInert is returned when a skill has neither instructions nor allowed tools
// and would have no effect when bound to an agent. DX-01-J (ACT-F3-04).
var ErrSkillInert = errors.New("skill: this skill has no instructions or tools and will have no effect")

// Deleter is a narrow interface for delete-with-binding-protection.
// Implemented by *Service; used by ManagementExecutor to ensure the service
// layer is called instead of the repository directly.
type Deleter interface {
	Delete(ctx context.Context, id uuid.UUID) error
}

// Service holds business logic for skills.
type Service struct {
	repo SkillRepository
}

// NewService creates a new Service.
func NewService(repo SkillRepository) *Service {
	return &Service{repo: repo}
}

// List returns a paginated list of skills, optionally filtered by category.
func (s *Service) List(ctx context.Context, category *string, req pagination.PageRequest) (pagination.Page[Response], error) {
	skills, total, err := s.repo.List(ctx, category, req)
	if err != nil {
		return pagination.Page[Response]{}, err
	}
	content := make([]Response, len(skills))
	for i, sk := range skills {
		content[i] = ResponseFrom(sk)
	}
	return pagination.NewPage(content, total, req), nil
}

// Create creates a new skill, auto-generating the slug if not provided.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Response, error) {
	// DX-01-H: normalize whitespace-only instructions so they don't appear as
	// non-empty but produce no LLM guidance.
	req.Instructions = strings.TrimSpace(req.Instructions)

	// ACT-F3-05: enforce maximum instructions size (32K chars).
	const maxInstructionsChars = 32000
	if len(req.Instructions) > maxInstructionsChars {
		return Response{}, fmt.Errorf("skill: instructions exceeds maximum length of %d chars (got %d)", maxInstructionsChars, len(req.Instructions))
	}

	// DX-01-J (ACT-F3-04): reject skills that are completely inert — no instructions
	// AND no tool restrictions means binding this skill to an agent has no effect.
	if req.Instructions == "" && len(req.AllowedTools) == 0 {
		return Response{}, ErrSkillInert
	}

	slug := req.Slug
	if slug == "" {
		slug = toSlug(req.Name)
	}

	// ensure slug uniqueness within tenant
	base := slug
	for i := 1; ; i++ {
		exists, err := s.repo.SlugExists(ctx, slug)
		if err != nil {
			return Response{}, fmt.Errorf("skill: check slug: %w", err)
		}
		if !exists {
			break
		}
		slug = fmt.Sprintf("%s_%d", base, i)
	}

	sk := Skill{
		Name:                   req.Name,
		Slug:                   slug,
		Description:            req.Description,
		Instructions:           req.Instructions,
		Category:               req.Category,
		AllowedTools:           req.AllowedTools,
		DisableModelInvocation: req.DisableModelInvocation,
		ContextMode:            req.ContextMode,
		WhenToUse:              req.WhenToUse,
		ArgumentHint:           req.ArgumentHint,
		ShouldDefer:            req.ShouldDefer,
	}
	created, err := s.repo.Create(ctx, sk)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(created), nil
}

// GetByID returns a skill by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Response, error) {
	sk, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(sk), nil
}

// Update updates a skill.
// Applies the same instruction-size and inert-skill guards as Create.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	// DX-01-H: normalize whitespace-only instructions.
	req.Instructions = strings.TrimSpace(req.Instructions)

	// ACT-F3-05: enforce maximum instructions size (32K chars).
	const maxInstructionsChars = 32000
	if len(req.Instructions) > maxInstructionsChars {
		return Response{}, fmt.Errorf("skill: instructions exceeds maximum length of %d chars (got %d)", maxInstructionsChars, len(req.Instructions))
	}

	// DX-01-J (ACT-F3-04): reject inert skill updates — no instructions AND no tool restrictions.
	if req.Instructions == "" && len(req.AllowedTools) == 0 {
		return Response{}, ErrSkillInert
	}

	sk, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(sk), nil
}

// Delete deletes a skill, but only if no agents currently reference it.
// Returns ErrSkillBoundToAgents when the skill is still in use.
// P-C185-1: prevents management executor from orphaning agent toolsets.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	count, err := s.repo.CountAgentBindings(ctx, id)
	if err != nil {
		return fmt.Errorf("skill: check bindings before delete: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w (agents bound: %d)", ErrSkillBoundToAgents, count)
	}
	return s.repo.Delete(ctx, id)
}

// toSlug converts a name to a kebab-case slug.
// "Document Search" → "document-search", "My  Tool!" → "my-tool"
func toSlug(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	// collapse consecutive hyphens
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
