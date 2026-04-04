package approval

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service defines the business logic for approval management.
type Service interface {
	Create(ctx context.Context, req CreateApprovalRequest) (PendingApproval, error)
	GetByID(ctx context.Context, id uuid.UUID) (PendingApproval, error)
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[PendingApproval], error)
	PendingCount(ctx context.Context) (PendingCountResponse, error)
	Respond(ctx context.Context, id uuid.UUID, respondedBy string, req RespondRequest) (PendingApproval, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repo Repository
}

// NewService creates a new approval service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, req CreateApprovalRequest) (PendingApproval, error) {
	if req.ExecutionID == uuid.Nil {
		return PendingApproval{}, fmt.Errorf("approval: executionId is required")
	}
	if req.NodeID == "" {
		return PendingApproval{}, fmt.Errorf("approval: nodeId is required")
	}
	if req.Title == "" {
		return PendingApproval{}, fmt.Errorf("approval: title is required")
	}
	return s.repo.Create(ctx, req)
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (PendingApproval, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[PendingApproval], error) {
	items, total, err := s.repo.List(ctx, req.Offset(), req.Size)
	if err != nil {
		return pagination.Page[PendingApproval]{}, err
	}
	return pagination.NewPage(items, total, req), nil
}

func (s *service) PendingCount(ctx context.Context) (PendingCountResponse, error) {
	count, err := s.repo.PendingCount(ctx)
	if err != nil {
		return PendingCountResponse{}, err
	}
	return PendingCountResponse{Count: count}, nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) Respond(ctx context.Context, id uuid.UUID, respondedBy string, req RespondRequest) (PendingApproval, error) {
	a, err := s.repo.Respond(ctx, id, respondedBy, req)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAlreadyResolved) {
		return PendingApproval{}, err
	}
	return a, err
}
