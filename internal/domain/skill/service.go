package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

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

	inputSchema := normaliseInputSchema(req.InputSchema)

	sk := Skill{
		Name:         req.Name,
		Slug:         slug,
		Description:  req.Description,
		Instructions: req.Instructions,
		Category:     req.Category,
		InputSchema:  inputSchema,
		OutputSchema: req.OutputSchema,
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
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	sk, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(sk), nil
}

// Delete deletes a skill.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// normaliseInputSchema converts array shorthand (["field1","field2"]) to a
// full JSON Schema object, and passes through existing object schemas unchanged.
// nil / empty input returns nil.
func normaliseInputSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return raw // not valid JSON — return as-is
	}
	arr, ok := parsed.([]any)
	if !ok {
		return raw // already an object (or string) — pass through
	}
	// Convert ["field1","field2"] → JSON Schema object with string properties
	properties := make(map[string]any, len(arr))
	required := make([]string, 0, len(arr))
	for _, item := range arr {
		if name, ok := item.(string); ok {
			properties[name] = map[string]any{"type": "string"}
			required = append(required, name)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
	out, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return out
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
