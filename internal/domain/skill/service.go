package skill

import (
	"context"
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

// List returns a paginated list of skills.
func (s *Service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[Response], error) {
	skills, total, err := s.repo.List(ctx, req)
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

	sk := Skill{
		Name:         req.Name,
		Slug:         slug,
		Description:  req.Description,
		Category:     req.Category,
		InputSchema:  req.InputSchema,
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

// toSlug converts a name to a snake_case slug.
func toSlug(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	// collapse consecutive underscores
	result := b.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return strings.Trim(result, "_")
}
