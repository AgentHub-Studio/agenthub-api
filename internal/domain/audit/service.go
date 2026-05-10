package audit

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// auditIdentPattern restringe entityType/action a chars seguros. Bug 253:
// ambos campos são renderizados na UI Audit Logs sem escape — sem este
// gate, "<script>" persistia no histórico de auditoria como vetor XSS
// permanente. Pattern alphanumeric + ._- cobre identificadores reais
// (agent, kb, chat-session, llm.preset, package_version).
var auditIdentPattern = regexp.MustCompile(`^[A-Za-z0-9_\-.]+$`)

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
	// Bug 131: entityType varchar(100), action varchar(50) — gate length
	// antes do INSERT (sem isso 500 SQL error 22001 vazava para o cliente).
	if len(req.EntityType) > 100 {
		return AuditLog{}, fmt.Errorf("%w: entityType exceeds maximum length of 100 chars (got %d)", ErrValidation, len(req.EntityType))
	}
	// Bug 253: entityType e action são renderizados na UI Audit Logs.
	// Sem pattern restritivo, "<script>" persistia → XSS na UI.
	// Pattern alphanumeric + ._- cobre nomes legítimos (agent, kb,
	// chat-session, llm.preset etc) e bloqueia chars perigosos.
	if !auditIdentPattern.MatchString(req.EntityType) {
		return AuditLog{}, fmt.Errorf("%w: entityType must match [A-Za-z0-9_\\-.]+ (got %q)", ErrValidation, req.EntityType)
	}
	if strings.TrimSpace(string(req.Action)) == "" {
		return AuditLog{}, fmt.Errorf("%w: action is required", ErrValidation)
	}
	if len(req.Action) > 50 {
		return AuditLog{}, fmt.Errorf("%w: action exceeds maximum length of 50 chars (got %d)", ErrValidation, len(req.Action))
	}
	// Bug 253: action também vai para UI; mesma proteção.
	if !auditIdentPattern.MatchString(string(req.Action)) {
		return AuditLog{}, fmt.Errorf("%w: action must match [A-Za-z0-9_\\-.]+ (got %q)", ErrValidation, req.Action)
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
