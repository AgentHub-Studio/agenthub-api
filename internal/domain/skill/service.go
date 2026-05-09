package skill

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// slugPattern enforces kebab-case (skill canonical format).
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Bug 179: stripHTML cross-cutting com agent/service.go (P-C280-1).
// Previne stored XSS quando description é renderizada na UI.
var htmlDangerousPattern = regexp.MustCompile(`(?is)<(script|style|iframe|object|embed|noscript)[^>]*>.*?</(script|style|iframe|object|embed|noscript)>`)
var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = htmlDangerousPattern.ReplaceAllString(s, "")
	s = htmlTagPattern.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

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

// ToolBinder is a narrow interface for auto-binding a tool to a skill after
// creation. BUG-F1 fix: allows POST /api/skills with {"toolId": "..."} to bind
// the tool in the same request rather than requiring a separate call.
type ToolBinder interface {
	// BindSkillTool binds toolID to skillID with default priority/active=true.
	BindSkillTool(ctx context.Context, skillID uuid.UUID, toolID uuid.UUID) error
}

// Service holds business logic for skills.
type Service struct {
	repo        SkillRepository
	toolBinder  ToolBinder // optional: enables toolId auto-bind on Create
}

// NewService creates a new Service.
func NewService(repo SkillRepository) *Service {
	return &Service{repo: repo}
}

// WithToolBinder injects a ToolBinder so Create can auto-bind a tool when toolId
// is provided in the request body.
func (s *Service) WithToolBinder(b ToolBinder) *Service {
	s.toolBinder = b
	return s
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
	// Bug 181: strip HTML do name (XSS prevention cross-cutting com agent).
	req.Name = stripHTML(req.Name)
	// Bug 130: name varchar(255) — gate length antes do INSERT
	// (sem este gate Create vazava SQL 22001 com 500).
	if len(req.Name) > 255 {
		return Response{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	// DX-01-H: normalize whitespace-only instructions so they don't appear as
	// non-empty but produce no LLM guidance.
	req.Instructions = strings.TrimSpace(req.Instructions)

	// ACT-F3-05: enforce maximum instructions size (32K chars).
	const maxInstructionsChars = 32000
	if len(req.Instructions) > maxInstructionsChars {
		return Response{}, fmt.Errorf("%w: instructions exceeds maximum length of %d chars (got %d)", ErrValidation, maxInstructionsChars, len(req.Instructions))
	}
	// Bug 159: cap description em 32KB (cross-cutting com agent/prompt-template).
	if len(req.Description) > 32000 {
		return Response{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	}
	// Bug 179: strip HTML do description (XSS prevention cross-cutting).
	req.Description = stripHTML(req.Description)

	// DX-01-J (ACT-F3-04): reject skills that are completely inert — no instructions
	// AND no tool restrictions means binding this skill to an agent has no effect.
	if req.Instructions == "" && len(req.AllowedTools) == 0 {
		return Response{}, ErrSkillInert
	}
	// Bug 169: cap allowedTools count em 100. Skills reais expõem
	// <20 tools; 1000+ é abuso e perf hit no prompt do LLM.
	if len(req.AllowedTools) > 100 {
		return Response{}, fmt.Errorf("%w: allowedTools exceeds maximum of 100 entries (got %d)", ErrValidation, len(req.AllowedTools))
	}

	// Bug 126: contextMode aceita só "inline" ou "fork" (varchar(10) na DB).
	// Sem este gate, valor inválido > 10 chars vazava SQL error 22001 (500)
	// para o cliente. Mesmo com tamanho válido, "INVALID_MODE" silencioso
	// quebrava o agentic runner que tem switch sobre os dois valores.
	if req.ContextMode != "" {
		switch req.ContextMode {
		case "inline", "fork":
		default:
			return Response{}, fmt.Errorf("%w: contextMode must be one of inline|fork (got %q)", ErrValidation, req.ContextMode)
		}
	}

	slug := req.Slug
	if slug == "" {
		slug = toSlug(req.Name)
	} else if !slugPattern.MatchString(slug) {
		return Response{}, fmt.Errorf("%w: slug must match [a-z0-9][a-z0-9-]* (got %q)", ErrValidation, slug)
	}
	// Bug 133: slug varchar(255) — gate length antes do INSERT
	// (slugPattern aceita qualquer tamanho desde que match os chars).
	if len(slug) > 255 {
		return Response{}, fmt.Errorf("%w: slug exceeds maximum length of 255 chars (got %d)", ErrValidation, len(slug))
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
	// Auto-bind tools when toolId / toolIds were provided in the request.
	// Non-fatal: skill was created; individual binding failures are ignored so that
	// the skill creation itself does not roll back (BUG-F1 / BUG-API-toolIds-IGNORED fix).
	if s.toolBinder != nil {
		if req.ToolID != nil && *req.ToolID != uuid.Nil {
			_ = s.toolBinder.BindSkillTool(ctx, created.ID, *req.ToolID)
		}
		for _, tid := range req.ToolIDs {
			if tid != uuid.Nil {
				_ = s.toolBinder.BindSkillTool(ctx, created.ID, tid)
			}
		}
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
	// BUG-SKILL-NAME-LOST fix: load existing skill and apply only non-zero fields
	// so that PATCH/PUT with a partial body does not wipe unchanged fields.
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Response{}, ErrNotFound
	}
	if req.Name == "" {
		req.Name = existing.Name
	} else if len(req.Name) > 255 {
		// Bug 137: name varchar(255) — gate length em Update.
		return Response{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	} else {
		// Bug 181: strip HTML do name (XSS prevention).
		req.Name = stripHTML(req.Name)
	}
	if req.Description == "" {
		req.Description = existing.Description
	} else if len(req.Description) > 32000 {
		// Bug 174: cap em Update (cross-cutting com Create — bug 159).
		return Response{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	} else {
		// Bug 179: strip HTML do description (XSS prevention).
		req.Description = stripHTML(req.Description)
	}
	if req.Category == "" {
		req.Category = existing.Category
	}
	if req.ContextMode == "" {
		req.ContextMode = existing.ContextMode
	} else {
		// Bug 126: gate enum em Update — mesmas razões do Create.
		switch req.ContextMode {
		case "inline", "fork":
		default:
			return Response{}, fmt.Errorf("%w: contextMode must be one of inline|fork (got %q)", ErrValidation, req.ContextMode)
		}
	}
	if req.WhenToUse == nil {
		req.WhenToUse = existing.WhenToUse
	}
	if req.ArgumentHint == nil {
		req.ArgumentHint = existing.ArgumentHint
	}
	if len(req.AllowedTools) == 0 {
		req.AllowedTools = existing.AllowedTools
	} else if len(req.AllowedTools) > 100 {
		// Bug 171: cap em Update (cross-cutting com Create — bug 169).
		return Response{}, fmt.Errorf("%w: allowedTools exceeds maximum of 100 entries (got %d)", ErrValidation, len(req.AllowedTools))
	}

	// DX-01-H: normalize whitespace-only instructions.
	req.Instructions = strings.TrimSpace(req.Instructions)
	// Bug 122: PATCH true-partial — quando o cliente omite "instructions",
	// preservar o valor atual em vez de tratar como "wipe". Sem este merge,
	// PATCH com apenas {"name":"..."} cai em ErrSkillInert porque Instructions
	// vira "" e AllowedTools (recém-mergeado) também é vazio para skills
	// instructions-only — false positive de inert.
	if req.Instructions == "" {
		req.Instructions = existing.Instructions
	}

	// ACT-F3-05: enforce maximum instructions size (32K chars).
	const maxInstructionsChars = 32000
	if len(req.Instructions) > maxInstructionsChars {
		return Response{}, fmt.Errorf("%w: instructions exceeds maximum length of %d chars (got %d)", ErrValidation, maxInstructionsChars, len(req.Instructions))
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
