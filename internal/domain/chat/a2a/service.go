package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type AgentRef struct {
	ID        uuid.UUID
	Name      string
	RunConfig *chat.AgentRunConfig
}

type AgentReader interface {
	GetA2AAgent(ctx context.Context, id uuid.UUID) (AgentRef, error)
}

// SessionExecutor creates and runs an isolated target-tenant chat session.
// chat.Service satisfies this interface.
type SessionExecutor interface {
	CreateSession(ctx context.Context, req chat.CreateSessionRequest) (chat.ChatSessionResponse, error)
	RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string, opts ...chat.RunSessionOptions) (<-chan chat.RunEvent, error)
}

// PreparedSessionExecutor is implemented by chat.Service. It allows A2A to
// reuse the snapshot from its newly persisted target session before that ID is
// visible to another caller.
type PreparedSessionExecutor interface {
	CreateAndRunSession(ctx context.Context, req chat.CreateSessionRequest, userMessage, tenantID string) (chat.PreparedSessionRun, error)
}

type Service interface {
	CreateGrant(ctx context.Context, ownerTenant string, req CreateGrantRequest) (GrantResponse, error)
	Invoke(ctx context.Context, sourceTenant string, req InvokeRequest) (InvokeResult, error)
}

type AuditRecorder interface {
	Record(ctx context.Context, tenantID string, req audit.RecordRequest) (audit.AuditLog, error)
}

type service struct {
	repo      Repository
	agents    AgentReader
	executor  SessionExecutor
	pairLimit int
	audit     AuditRecorder
}

func NewService(repo Repository, agents AgentReader, executor SessionExecutor) *service {
	return &service{
		repo:      repo,
		agents:    agents,
		executor:  executor,
		pairLimit: defaultPairLimit,
	}
}

func (s *service) WithAuditRecorder(recorder AuditRecorder) *service {
	s.audit = recorder
	return s
}

func (s *service) WithPairLimit(limit int) *service {
	s.pairLimit = normalizePairLimit(limit)
	return s
}

func (s *service) CreateGrant(ctx context.Context, ownerTenant string, req CreateGrantRequest) (GrantResponse, error) {
	ownerTenant = strings.TrimSpace(ownerTenant)
	if ownerTenant == "" {
		return GrantResponse{}, fmt.Errorf("%w: owner tenant is required", ErrValidation)
	}
	subjectTenant := strings.TrimSpace(req.SubjectTenant)
	if subjectTenant == "" {
		return GrantResponse{}, fmt.Errorf("%w: subjectTenant is required", ErrValidation)
	}
	if subjectTenant == ownerTenant {
		return GrantResponse{}, fmt.Errorf("%w: subject and owner tenants must differ", ErrValidation)
	}
	agentID, err := uuid.Parse(strings.TrimSpace(req.AgentID))
	if err != nil {
		return GrantResponse{}, fmt.Errorf("%w: invalid agentId", ErrValidation)
	}
	actions, err := normalizeActions(req.Actions)
	if err != nil {
		return GrantResponse{}, fmt.Errorf("%w: unsupported action", ErrValidation)
	}
	if len(actions) == 0 {
		return GrantResponse{}, fmt.Errorf("%w: actions are required", ErrValidation)
	}

	targetCtx := tenantctx.NewContext(ctx, ownerTenant)
	if _, err := s.agents.GetA2AAgent(targetCtx, agentID); err != nil {
		return GrantResponse{}, err
	}

	grant, err := s.repo.UpsertGrant(ctx, ownerTenant, Grant{
		SubjectTenant: subjectTenant,
		AgentID:       agentID,
		Actions:       actions,
	})
	if err != nil {
		return GrantResponse{}, err
	}
	s.recordGrantAudit(ctx, ownerTenant, grant)
	return GrantResponseFrom(grant), nil
}

func (s *service) Invoke(ctx context.Context, sourceTenant string, req InvokeRequest) (InvokeResult, error) {
	preparationStarted := time.Now()
	var preparation InvokePreparationTimings
	sourceTenant = strings.TrimSpace(sourceTenant)
	targetTenant := strings.TrimSpace(req.TargetTenant)
	if sourceTenant == "" || targetTenant == "" {
		return InvokeResult{}, fmt.Errorf("%w: source and target tenants are required", ErrValidation)
	}
	if sourceTenant == targetTenant {
		return InvokeResult{}, fmt.Errorf("%w: source and target tenants must differ", ErrValidation)
	}
	agentID, err := uuid.Parse(strings.TrimSpace(req.AgentID))
	if err != nil {
		return InvokeResult{}, fmt.Errorf("%w: invalid agentId", ErrValidation)
	}

	stageStarted := time.Now()
	allowed, err := s.repo.HasGrant(ctx, targetTenant, sourceTenant, agentID, ActionInvoke)
	preparation.GrantCheckMS = time.Since(stageStarted).Milliseconds()
	if err != nil {
		return InvokeResult{}, err
	}
	if !allowed {
		return InvokeResult{}, ErrForbidden
	}
	if s.executor == nil {
		return InvokeResult{}, fmt.Errorf("a2a: target session executor is not configured")
	}
	stageStarted = time.Now()
	withinLimit, _, err := s.repo.ConsumeRateLimit(ctx, targetTenant, sourceTenant, time.Now().UTC(), defaultPairLimitWindow, s.pairLimit)
	preparation.RateLimitMS = time.Since(stageStarted).Milliseconds()
	if err != nil {
		return InvokeResult{}, err
	}
	if !withinLimit {
		return InvokeResult{}, ErrRateLimited
	}

	targetCtx, cancelTarget := isolatedTargetContext(ctx, targetTenant)
	stageStarted = time.Now()
	targetAgent, err := s.agents.GetA2AAgent(targetCtx, agentID)
	preparation.AgentLoadMS = time.Since(stageStarted).Milliseconds()
	if err != nil {
		cancelTarget()
		return InvokeResult{}, err
	}

	input := strings.TrimSpace(req.Input)
	if input == "" {
		input = "ping"
	}
	sessionRequest := chat.CreateSessionRequest{
		AgentID:     &targetAgent.ID,
		Title:       "A2A invocation",
		AgentConfig: targetAgent.RunConfig,
	}

	var session chat.ChatSessionResponse
	var events <-chan chat.RunEvent
	if preparedExecutor, ok := s.executor.(PreparedSessionExecutor); ok {
		preparedRun, err := preparedExecutor.CreateAndRunSession(targetCtx, sessionRequest, input, targetTenant)
		if err != nil {
			cancelTarget()
			return InvokeResult{}, fmt.Errorf("a2a: create and run target session: %w", err)
		}
		session = preparedRun.Session
		events = preparedRun.Events
		preparation.SessionCreateMS = preparedRun.SessionCreateMS
		preparation.RunPrepareMS = preparedRun.RunPrepareMS
	} else {
		stageStarted = time.Now()
		session, err = s.executor.CreateSession(targetCtx, sessionRequest)
		preparation.SessionCreateMS = time.Since(stageStarted).Milliseconds()
		if err != nil {
			cancelTarget()
			return InvokeResult{}, fmt.Errorf("a2a: create target session: %w", err)
		}

		stageStarted = time.Now()
		events, err = s.executor.RunSession(targetCtx, session.ID, input, targetTenant, chat.RunSessionOptions{
			AgentConfig: targetAgent.RunConfig,
		})
		preparation.RunPrepareMS = time.Since(stageStarted).Milliseconds()
		if err != nil {
			cancelTarget()
			return InvokeResult{}, fmt.Errorf("a2a: run target session: %w", err)
		}
	}

	preparation.TotalMS = time.Since(preparationStarted).Milliseconds()
	result := InvokeResult{
		SessionID:    session.ID,
		TargetTenant: targetTenant,
		AgentID:      agentID,
		Events:       events,
		Preparation:  preparation,
		cancel:       cancelTarget,
	}
	s.recordInvokeAudit(ctx, targetTenant, sourceTenant, result)
	return result, nil
}

// isolatedTargetContext retains only cancellation from the source request. It
// deliberately starts from Background so the source tenant's raw JWT and
// request-scoped values cannot reach the target agent or its tools.
func isolatedTargetContext(source context.Context, targetTenant string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(tenantctx.NewContextWithToken(context.Background(), targetTenant, ""))
	stop := context.AfterFunc(source, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

func (s *service) recordGrantAudit(ctx context.Context, ownerTenant string, grant Grant) {
	if s.audit == nil {
		return
	}
	metadata, _ := json.Marshal(map[string]any{
		"subjectTenant": grant.SubjectTenant,
		"agentId":       grant.AgentID.String(),
		"actions":       grant.Actions,
	})
	if _, err := s.audit.Record(ctx, ownerTenant, audit.RecordRequest{
		EntityType: "a2a_grant",
		EntityID:   grant.ID.String(),
		Action:     audit.AuditActionCreate,
		Metadata:   string(metadata),
	}); err != nil {
		slog.Warn("a2a: failed to record grant audit",
			"tenantID", ownerTenant,
			"grantID", grant.ID,
			"error", err)
	}
}

func (s *service) recordInvokeAudit(ctx context.Context, targetTenant, sourceTenant string, result InvokeResult) {
	if s.audit == nil {
		return
	}
	metadata, _ := json.Marshal(map[string]string{
		"sourceTenant": sourceTenant,
		"targetTenant": targetTenant,
		"agentId":      result.AgentID.String(),
		"sessionId":    result.SessionID.String(),
	})
	req := audit.RecordRequest{
		EntityType: "a2a_invoke",
		EntityID:   result.AgentID.String(),
		Action:     audit.AuditActionExecute,
		Metadata:   string(metadata),
	}
	go s.recordInvokeAuditBestEffort(ctx, targetTenant, sourceTenant, result.AgentID, req)
}

func (s *service) recordInvokeAuditBestEffort(ctx context.Context, targetTenant, sourceTenant string, agentID uuid.UUID, req audit.RecordRequest) {
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultAuditTimeout)
	defer cancel()
	if _, err := s.audit.Record(auditCtx, targetTenant, req); err != nil {
		slog.Warn("a2a: failed to record invoke audit",
			"tenantID", targetTenant,
			"sourceTenant", sourceTenant,
			"agentID", agentID,
			"error", err)
	}
}

func normalizeActions(actions []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, action := range actions {
		action = strings.TrimSpace(strings.ToLower(action))
		if action == "" || seen[action] {
			continue
		}
		// The ADR currently defines only invoke. Rejecting unsupported values
		// prevents a grant from implicitly authorizing a future action.
		if action != ActionInvoke {
			return nil, fmt.Errorf("unsupported action %q", action)
		}
		seen[action] = true
		out = append(out, action)
	}
	return out, nil
}
