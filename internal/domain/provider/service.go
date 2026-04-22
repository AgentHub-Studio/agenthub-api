package provider

import (
	"context"
	"fmt"
)

// Service is the business-logic layer for the provider catalog.
type Service struct {
	repo Repository
}

// NewService returns a Service wired to the given Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListAll returns every enabled provider, optionally filtered by kind.
// An empty kind returns all providers.
func (s *Service) ListAll(ctx context.Context, kind string) ([]Response, error) {
	var (
		providers []Provider
		err       error
	)
	if kind == "" {
		providers, err = s.repo.ListAll(ctx)
	} else {
		providers, err = s.repo.ListByKind(ctx, Kind(kind))
	}
	if err != nil {
		return nil, fmt.Errorf("provider service list: %w", err)
	}
	out := make([]Response, 0, len(providers))
	for _, p := range providers {
		out = append(out, ResponseFrom(p))
	}
	return out, nil
}

// GetBySlug returns a single provider by its slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (Response, error) {
	p, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return Response{}, err
	}
	return ResponseFrom(p), nil
}
