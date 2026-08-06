package skill

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
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

// ToolBinder is a narrow interface for auto-binding a tool to a skill after
// creation. BUG-F1 fix: allows POST /api/skills with {"toolId": "..."} to bind
// the tool in the same request rather than requiring a separate call.
type ToolBinder interface {
	// BindSkillTool binds toolID to skillID with default priority/active=true.
	BindSkillTool(ctx context.Context, skillID uuid.UUID, toolID uuid.UUID) error
}

// Service holds business logic for skills.
type Service struct {
	repo       SkillRepository
	toolBinder ToolBinder // optional: enables toolId auto-bind on Create
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
	req.Name = strings.TrimSpace(req.Name)
	if sanitize.ContainsHTML(req.Name) {
		return Response{}, fmt.Errorf("%w: name must not contain HTML tags", ErrValidation)
	}
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
	req.Description = sanitize.StripHTML(req.Description)
	var err error
	req.RequiredRoles, err = normalizeRequiredRoles(req.RequiredRoles)
	if err != nil {
		return Response{}, err
	}
	req.ModelOverrides, err = normalizeModelOverrides(req.ModelOverrides)
	if err != nil {
		return Response{}, err
	}
	req.EffortLevel, err = normalizeEffortLevel(req.EffortLevel)
	if err != nil {
		return Response{}, err
	}
	req.AssociatedAgents, err = normalizeSkillSlugs(req.AssociatedAgents, "associatedAgents")
	if err != nil {
		return Response{}, err
	}
	req.DynamicHooks, err = normalizeHookEvents(req.DynamicHooks)
	if err != nil {
		return Response{}, err
	}

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
	} else {
		slug = strings.TrimSpace(slug)
		if !sanitize.ValidSlug(slug) {
			return Response{}, fmt.Errorf("%w: slug must match %s (got %q)", ErrValidation, sanitize.CanonicalSlugPattern, slug)
		}
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
		slug = sanitize.SlugWithNumericSuffix(base, i)
	}

	sk := Skill{
		Name:                   req.Name,
		Slug:                   slug,
		Description:            req.Description,
		Instructions:           req.Instructions,
		Category:               req.Category,
		AllowedTools:           req.AllowedTools,
		RequiredRoles:          req.RequiredRoles,
		DisableModelInvocation: req.DisableModelInvocation,
		ContextMode:            req.ContextMode,
		WhenToUse:              req.WhenToUse,
		ArgumentHint:           req.ArgumentHint,
		ShouldDefer:            req.ShouldDefer,
		ModelOverrides:         req.ModelOverrides,
		EffortLevel:            req.EffortLevel,
		AssociatedAgents:       req.AssociatedAgents,
		DynamicHooks:           req.DynamicHooks,
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
	} else if sanitize.ContainsHTML(req.Name) {
		return Response{}, fmt.Errorf("%w: name must not contain HTML tags", ErrValidation)
	} else if len(strings.TrimSpace(req.Name)) > 255 {
		// Bug 137: name varchar(255) — gate length em Update.
		return Response{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(strings.TrimSpace(req.Name)))
	} else {
		req.Name = strings.TrimSpace(req.Name)
	}
	if req.Description == "" {
		req.Description = existing.Description
	} else if len(req.Description) > 32000 {
		// Bug 174: cap em Update (cross-cutting com Create — bug 159).
		return Response{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	} else {
		// Bug 179: strip HTML do description (XSS prevention).
		req.Description = sanitize.StripHTML(req.Description)
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
	if len(req.RequiredRoles) == 0 {
		req.RequiredRoles = existing.RequiredRoles
	} else {
		normalized, err := normalizeRequiredRoles(req.RequiredRoles)
		if err != nil {
			return Response{}, err
		}
		req.RequiredRoles = normalized
	}
	if len(req.ModelOverrides) == 0 {
		req.ModelOverrides = existing.ModelOverrides
	} else {
		normalized, err := normalizeModelOverrides(req.ModelOverrides)
		if err != nil {
			return Response{}, err
		}
		req.ModelOverrides = normalized
	}
	if req.EffortLevel == "" {
		req.EffortLevel = existing.EffortLevel
	} else {
		normalized, err := normalizeEffortLevel(req.EffortLevel)
		if err != nil {
			return Response{}, err
		}
		req.EffortLevel = normalized
	}
	if len(req.AssociatedAgents) == 0 {
		req.AssociatedAgents = existing.AssociatedAgents
	} else {
		normalized, err := normalizeSkillSlugs(req.AssociatedAgents, "associatedAgents")
		if err != nil {
			return Response{}, err
		}
		req.AssociatedAgents = normalized
	}
	if len(req.DynamicHooks) == 0 {
		req.DynamicHooks = existing.DynamicHooks
	} else {
		normalized, err := normalizeHookEvents(req.DynamicHooks)
		if err != nil {
			return Response{}, err
		}
		req.DynamicHooks = normalized
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
	return sanitize.ToSlug(name, "skill")
}

func normalizeRequiredRoles(roles []string) ([]string, error) {
	if len(roles) == 0 {
		return []string{}, nil
	}
	if len(roles) > 100 {
		return nil, fmt.Errorf("%w: requiredRoles exceeds maximum of 100 entries (got %d)", ErrValidation, len(roles))
	}
	seen := make(map[string]struct{}, len(roles))
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			return nil, fmt.Errorf("%w: requiredRoles must not contain empty values", ErrValidation)
		}
		if len(role) > 128 {
			return nil, fmt.Errorf("%w: requiredRoles entry exceeds maximum length of 128 chars (got %d)", ErrValidation, len(role))
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out, nil
}

func normalizeModelOverrides(overrides map[string]string) (map[string]string, error) {
	if len(overrides) == 0 {
		return map[string]string{}, nil
	}
	if len(overrides) > 20 {
		return nil, fmt.Errorf("%w: modelOverrides exceeds maximum of 20 entries", ErrValidation)
	}
	normalized := make(map[string]string, len(overrides))
	for contextName, model := range overrides {
		contextName = strings.TrimSpace(contextName)
		model = strings.TrimSpace(model)
		if contextName == "" || model == "" || len(contextName) > 64 || len(model) > 255 {
			return nil, fmt.Errorf("%w: modelOverrides contains an invalid entry", ErrValidation)
		}
		normalized[contextName] = model
	}
	return normalized, nil
}

func normalizeEffortLevel(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "lowest", "low", "medium", "high", "highest":
		return value, nil
	default:
		return "", fmt.Errorf("%w: effortLevel must be lowest|low|medium|high|highest", ErrValidation)
	}
}

func normalizeSkillSlugs(values []string, field string) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}
	if len(values) > 100 {
		return nil, fmt.Errorf("%w: %s exceeds maximum of 100 entries", ErrValidation, field)
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !sanitize.ValidSlug(value) {
			return nil, fmt.Errorf("%w: %s must contain canonical slugs", ErrValidation, field)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeHookEvents(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}
	if len(values) > 100 {
		return nil, fmt.Errorf("%w: dynamicHooks exceeds maximum of 100 entries", ErrValidation)
	}
	allowed := map[string]struct{}{
		"pre_tool_use": {}, "post_tool_use": {}, "post_tool_failure": {},
		"session_start": {}, "session_end": {}, "notification": {},
		"turn_end": {}, "run_end": {},
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, ok := allowed[value]; !ok {
			return nil, fmt.Errorf("%w: dynamicHooks contains unsupported hook event %q", ErrValidation, value)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}
