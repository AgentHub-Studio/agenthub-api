package abtest

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service manages A/B tests and session variant assignment.
type Service interface {
	List(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[ABTestResponse], error)
	GetByID(ctx context.Context, id uuid.UUID) (ABTestResponse, error)
	Create(ctx context.Context, req CreateABTestRequest) (ABTestResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateABTestRequest) (ABTestResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// AssignVariant looks up the active test for the agent and probabilistically
	// assigns the session to either "control" or "variant".
	// Returns the VariantVersionID to use (nil = use the agent's live config).
	// Records the assignment for later analysis.
	AssignVariant(ctx context.Context, agentID, sessionID uuid.UUID) (*uuid.UUID, error)
}

type service struct {
	repo Repository
}

// NewService creates a Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[ABTestResponse], error) {
	page, err := s.repo.List(ctx, agentID, req)
	if err != nil {
		return pagination.Page[ABTestResponse]{}, err
	}
	return pagination.MapPage(page, responseFrom), nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (ABTestResponse, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ABTestResponse{}, err
	}
	return responseFrom(t), nil
}

func (s *service) Create(ctx context.Context, req CreateABTestRequest) (ABTestResponse, error) {
	if err := req.validate(); err != nil {
		return ABTestResponse{}, err
	}
	t := ABTest{
		AgentID:          req.AgentID,
		Name:             req.Name,
		Description:      req.Description,
		ControlVersionID: req.ControlVersionID,
		VariantVersionID: req.VariantVersionID,
		TrafficPercent:   req.TrafficPercent,
		Status:           TestStatusActive,
		StartedAt:        time.Now(),
	}
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return ABTestResponse{}, err
	}
	return responseFrom(created), nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateABTestRequest) (ABTestResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ABTestResponse{}, err
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.TrafficPercent != nil {
		if *req.TrafficPercent < 0 || *req.TrafficPercent > 100 {
			return ABTestResponse{}, fmt.Errorf("abtest: traffic_percent must be 0–100")
		}
		existing.TrafficPercent = *req.TrafficPercent
	}
	if req.Status != nil {
		existing.Status = TestStatus(*req.Status)
		if existing.Status == TestStatusConcluded && existing.EndedAt == nil {
			now := time.Now()
			existing.EndedAt = &now
		}
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return ABTestResponse{}, err
	}
	return responseFrom(updated), nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) AssignVariant(ctx context.Context, agentID, sessionID uuid.UUID) (*uuid.UUID, error) {
	test, err := s.repo.GetActiveByAgent(ctx, agentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil // no active test — use control
		}
		return nil, fmt.Errorf("abtest: assign variant: %w", err)
	}

	roll := randomPercent()
	var variant Variant
	var result *uuid.UUID

	if roll < test.TrafficPercent {
		variant = VariantVariant
		result = &test.VariantVersionID
	} else {
		variant = VariantControl
		result = test.ControlVersionID // may be nil
	}

	if err := s.repo.RecordAssignment(ctx, Assignment{
		TestID:    test.ID,
		SessionID: sessionID,
		Variant:   variant,
	}); err != nil {
		// Non-fatal: log but don't block the session.
		_ = err
	}

	return result, nil
}

// randomPercent returns a pseudo-random number in [0,100) using crypto/rand.
func randomPercent() int {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 50 // safe fallback
	}
	n := binary.BigEndian.Uint64(b[:])
	return int(n % 100)
}
