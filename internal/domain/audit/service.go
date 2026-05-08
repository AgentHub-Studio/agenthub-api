package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// LogRepository defines the persistence interface for AuditLog.
type LogRepository interface {
	ListAll(ctx context.Context, tenantID string, f ListFilter, pr pagination.PageRequest) ([]AuditLog, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (AuditLog, error)
	Record(ctx context.Context, tenantID string, l AuditLog) (AuditLog, error)
}

// Service implements business logic for audit logs.
type Service struct {
	repo LogRepository
}

// NewService creates a new Service.
func NewService(repo LogRepository) *Service {
	return &Service{repo: repo}
}

// ListAll returns a paginated, filtered list of audit logs.
func (s *Service) ListAll(ctx context.Context, tenantID string, f ListFilter, pr pagination.PageRequest) ([]AuditLog, int, error) {
	return s.repo.ListAll(ctx, tenantID, f, pr)
}

// GetByID retrieves an audit log entry by ID.
func (s *Service) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (AuditLog, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// Record appends a new audit log entry.
func (s *Service) Record(ctx context.Context, tenantID string, req RecordRequest) (AuditLog, error) {
	if strings.TrimSpace(req.EntityType) == "" {
		return AuditLog{}, fmt.Errorf("%w: entityType is required", ErrValidation)
	}
	if strings.TrimSpace(string(req.Action)) == "" {
		return AuditLog{}, fmt.Errorf("%w: action is required", ErrValidation)
	}
	l := AuditLog{
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		Action:     req.Action,
		ActorID:    req.ActorID,
		ActorEmail: req.ActorEmail,
		OldValue:   req.OldValue,
		NewValue:   req.NewValue,
		Metadata:   req.Metadata,
		IPAddress:  req.IPAddress,
	}
	return s.repo.Record(ctx, tenantID, l)
}
