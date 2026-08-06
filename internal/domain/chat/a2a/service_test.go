package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type fakeRepository struct {
	grants map[string]Grant
	limits map[string]int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		grants: map[string]Grant{},
		limits: map[string]int{},
	}
}

func (r *fakeRepository) UpsertGrant(_ context.Context, ownerTenant string, g Grant) (Grant, error) {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	now := time.Now().UTC()
	g.CreatedAt = now
	g.UpdatedAt = now
	r.grants[grantKey(ownerTenant, g.SubjectTenant, g.AgentID)] = g
	return g, nil
}

func (r *fakeRepository) HasGrant(_ context.Context, ownerTenant string, subjectTenant string, agentID uuid.UUID, action string) (bool, error) {
	g, ok := r.grants[grantKey(ownerTenant, subjectTenant, agentID)]
	if !ok {
		return false, nil
	}
	for _, allowed := range g.Actions {
		if allowed == action || allowed == "*" {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepository) ConsumeRateLimit(_ context.Context, targetTenant string, sourceTenant string, now time.Time, window time.Duration, limit int) (bool, int, error) {
	key := targetTenant + "|" + sourceTenant + "|" + rateLimitWindowStart(now, window).Format(time.RFC3339)
	r.limits[key]++
	count := r.limits[key]
	return count <= normalizePairLimit(limit), count, nil
}

func grantKey(ownerTenant, subjectTenant string, agentID uuid.UUID) string {
	return ownerTenant + "|" + subjectTenant + "|" + agentID.String()
}

type fakeAgentReader struct {
	agents map[string]map[uuid.UUID]AgentRef
	seen   []string
}

func newFakeAgentReader() *fakeAgentReader {
	return &fakeAgentReader{agents: map[string]map[uuid.UUID]AgentRef{}}
}

func (r *fakeAgentReader) add(tenant string, agent AgentRef) {
	if r.agents[tenant] == nil {
		r.agents[tenant] = map[uuid.UUID]AgentRef{}
	}
	r.agents[tenant][agent.ID] = agent
}

func (r *fakeAgentReader) GetA2AAgent(ctx context.Context, id uuid.UUID) (AgentRef, error) {
	tenant := tenantctx.FromContext(ctx)
	r.seen = append(r.seen, tenant)
	if agent, ok := r.agents[tenant][id]; ok {
		return agent, nil
	}
	return AgentRef{}, ErrNotFound
}

type fakeSessionExecutor struct {
	createContext context.Context
	createRequest chat.CreateSessionRequest
	runContext    context.Context
	runSessionID  uuid.UUID
	runInput      string
	runTenant     string
	runOptions    []chat.RunSessionOptions
	events        <-chan chat.RunEvent
	createErr     error
	runErr        error
}

func newFakeSessionExecutor() *fakeSessionExecutor {
	events := make(chan chat.RunEvent)
	close(events)
	return &fakeSessionExecutor{events: events}
}

func (e *fakeSessionExecutor) CreateSession(ctx context.Context, req chat.CreateSessionRequest) (chat.ChatSessionResponse, error) {
	e.createContext = ctx
	e.createRequest = req
	if e.createErr != nil {
		return chat.ChatSessionResponse{}, e.createErr
	}
	return chat.ChatSessionResponse{ID: uuid.New()}, nil
}

func (e *fakeSessionExecutor) RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string, opts ...chat.RunSessionOptions) (<-chan chat.RunEvent, error) {
	e.runContext = ctx
	e.runSessionID = sessionID
	e.runInput = userMessage
	e.runTenant = tenantID
	e.runOptions = append([]chat.RunSessionOptions(nil), opts...)
	if e.runErr != nil {
		return nil, e.runErr
	}
	return e.events, nil
}

type fakeAuditRecorder struct {
	mu       sync.Mutex
	tenantID string
	requests []audit.RecordRequest
	err      error
	delay    time.Duration
}

func (r *fakeAuditRecorder) Record(ctx context.Context, tenantID string, req audit.RecordRequest) (audit.AuditLog, error) {
	if r.delay > 0 {
		timer := time.NewTimer(r.delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return audit.AuditLog{}, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenantID = tenantID
	r.requests = append(r.requests, req)
	return audit.AuditLog{ID: uuid.New()}, r.err
}

func (r *fakeAuditRecorder) snapshot() (string, []audit.RecordRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := append([]audit.RecordRequest(nil), r.requests...)
	return r.tenantID, requests
}

func TestServiceCreateGrantStoresConsentInOwnerTenant(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})

	svc := NewService(repo, agents, nil)
	resp, err := svc.CreateGrant(context.Background(), " tenant-b ", CreateGrantRequest{
		SubjectTenant: " tenant-a ",
		AgentID:       agentID.String(),
		Actions:       []string{"invoke", "INVOKE", ""},
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "tenant-a", resp.SubjectTenant)
	assert.Equal(t, []string{ActionInvoke}, resp.Actions)
	assert.Equal(t, []string{"tenant-b"}, agents.seen)
	_, exists := repo.grants[grantKey("tenant-b", "tenant-a", agentID)]
	assert.True(t, exists)
}

func TestServiceCreateGrantRejectsUnsupportedActionsBeforeAnySideEffect(t *testing.T) {
	for name, actions := range map[string][]string{
		"wildcard":          {"*"},
		"unknown":           {"delegate"},
		"mixed with invoke": {ActionInvoke, "delegate"},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepository()
			agents := newFakeAgentReader()
			agentID := uuid.New()
			agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})

			_, err := NewService(repo, agents, nil).CreateGrant(context.Background(), "tenant-b", CreateGrantRequest{
				SubjectTenant: "tenant-a",
				AgentID:       agentID.String(),
				Actions:       actions,
			})

			require.ErrorIs(t, err, ErrValidation)
			assert.Empty(t, agents.seen)
			assert.Empty(t, repo.grants)
		})
	}
}

func TestServiceCreateGrantRequiresDifferentSubjectTenant(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})

	_, err := NewService(repo, agents, nil).CreateGrant(context.Background(), " tenant-b ", CreateGrantRequest{
		SubjectTenant: " tenant-b ",
		AgentID:       agentID.String(),
		Actions:       []string{ActionInvoke},
	})

	require.ErrorIs(t, err, ErrValidation)
	assert.Empty(t, agents.seen)
	assert.Empty(t, repo.grants)
}

func TestServiceCreateGrantRecordsOperationalAudit(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	auditRecorder := &fakeAuditRecorder{}
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})

	svc := NewService(repo, agents, nil).WithAuditRecorder(auditRecorder)
	resp, err := svc.CreateGrant(context.Background(), "tenant-b", CreateGrantRequest{
		SubjectTenant: "tenant-a",
		AgentID:       agentID.String(),
		Actions:       []string{ActionInvoke},
	})

	require.NoError(t, err)
	tenantID, requests := auditRecorder.snapshot()
	require.Len(t, requests, 1)
	req := requests[0]
	assert.Equal(t, "tenant-b", tenantID)
	assert.Equal(t, "a2a_grant", req.EntityType)
	assert.Equal(t, resp.ID, req.EntityID)
	assert.Equal(t, audit.AuditActionCreate, req.Action)
	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(req.Metadata), &metadata))
	assert.Equal(t, "tenant-a", metadata["subjectTenant"])
	assert.Equal(t, agentID.String(), metadata["agentId"])
	assert.ElementsMatch(t, []any{ActionInvoke}, metadata["actions"].([]any))
}

func TestServiceInvokeRequiresGrant(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})

	svc := NewService(repo, agents, nil)
	_, err := svc.Invoke(context.Background(), "tenant-a", InvokeRequest{
		TargetTenant: "tenant-b",
		AgentID:      agentID.String(),
		Input:        "hello",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrForbidden))
	assert.Empty(t, agents.seen)
}

func TestServiceInvokeUsesTargetTenantAndRateLimit(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	agentID := uuid.New()
	config := &chat.AgentRunConfig{ID: agentID, Name: "Target Agent", Status: "PUBLISHED"}
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent", RunConfig: config})
	_, err := repo.UpsertGrant(context.Background(), "tenant-b", Grant{
		SubjectTenant: "tenant-a",
		AgentID:       agentID,
		Actions:       []string{ActionInvoke},
	})
	require.NoError(t, err)

	executor := newFakeSessionExecutor()
	svc := NewService(repo, agents, executor).WithPairLimit(2)

	for i := 0; i < 2; i++ {
		resp, err := svc.Invoke(context.Background(), "tenant-a", InvokeRequest{
			TargetTenant: "tenant-b",
			AgentID:      agentID.String(),
			Input:        "ping",
		})
		require.NoError(t, err)
		assert.Equal(t, "tenant-b", resp.TargetTenant)
		assert.NotEqual(t, uuid.Nil, resp.SessionID)
		assert.GreaterOrEqual(t, resp.Preparation.TotalMS, resp.Preparation.GrantCheckMS)
		assert.GreaterOrEqual(t, resp.Preparation.TotalMS, resp.Preparation.RateLimitMS)
		assert.GreaterOrEqual(t, resp.Preparation.TotalMS, resp.Preparation.AgentLoadMS)
		assert.GreaterOrEqual(t, resp.Preparation.TotalMS, resp.Preparation.SessionCreateMS)
		assert.GreaterOrEqual(t, resp.Preparation.TotalMS, resp.Preparation.RunPrepareMS)
		resp.Cancel()
	}

	_, err = svc.Invoke(context.Background(), "tenant-a", InvokeRequest{
		TargetTenant: "tenant-b",
		AgentID:      agentID.String(),
		Input:        "ping",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimited))
	assert.Equal(t, []string{"tenant-b", "tenant-b"}, agents.seen)
	assert.Equal(t, "tenant-b", tenantctx.FromContext(executor.runContext))
	assert.Empty(t, tenantctx.TokenFromContext(executor.runContext))
	assert.Equal(t, "ping", executor.runInput)
	assert.Equal(t, "tenant-b", executor.runTenant)
	require.NotNil(t, executor.createRequest.AgentID)
	assert.Equal(t, agentID, *executor.createRequest.AgentID)
	assert.Same(t, config, executor.createRequest.AgentConfig)
	require.Len(t, executor.runOptions, 1)
	assert.Same(t, config, executor.runOptions[0].AgentConfig)
}

func TestServiceInvokeRateLimitIsSharedAcrossServiceInstances(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})
	_, err := repo.UpsertGrant(context.Background(), "tenant-b", Grant{
		SubjectTenant: "tenant-a",
		AgentID:       agentID,
		Actions:       []string{ActionInvoke},
	})
	require.NoError(t, err)

	executor := newFakeSessionExecutor()
	first := NewService(repo, agents, executor).WithPairLimit(2)
	second := NewService(repo, agents, executor).WithPairLimit(2)

	for i := 0; i < 2; i++ {
		resp, err := first.Invoke(context.Background(), "tenant-a", InvokeRequest{
			TargetTenant: "tenant-b",
			AgentID:      agentID.String(),
			Input:        "ping",
		})
		require.NoError(t, err)
		resp.Cancel()
	}

	_, err = second.Invoke(context.Background(), "tenant-a", InvokeRequest{
		TargetTenant: "tenant-b",
		AgentID:      agentID.String(),
		Input:        "ping",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimited))
}

func TestServiceInvokeRecordsOperationalAuditWithoutBlockingOnAuditFailure(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	auditRecorder := &fakeAuditRecorder{err: errors.New("audit down")}
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})
	_, err := repo.UpsertGrant(context.Background(), "tenant-b", Grant{
		SubjectTenant: "tenant-a",
		AgentID:       agentID,
		Actions:       []string{ActionInvoke},
	})
	require.NoError(t, err)

	executor := newFakeSessionExecutor()
	svc := NewService(repo, agents, executor).WithAuditRecorder(auditRecorder)
	resp, err := svc.Invoke(context.Background(), "tenant-a", InvokeRequest{
		TargetTenant: "tenant-b",
		AgentID:      agentID.String(),
		Input:        "hello",
	})

	require.NoError(t, err)
	resp.Cancel()
	require.Eventually(t, func() bool {
		_, requests := auditRecorder.snapshot()
		return len(requests) == 1
	}, time.Second, 10*time.Millisecond)
	tenantID, requests := auditRecorder.snapshot()
	req := requests[0]
	assert.Equal(t, "tenant-b", tenantID)
	assert.Equal(t, "a2a_invoke", req.EntityType)
	assert.Equal(t, agentID.String(), req.EntityID)
	assert.Equal(t, audit.AuditActionExecute, req.Action)
	var metadata map[string]string
	require.NoError(t, json.Unmarshal([]byte(req.Metadata), &metadata))
	assert.Equal(t, "tenant-a", metadata["sourceTenant"])
	assert.Equal(t, "tenant-b", metadata["targetTenant"])
	assert.Equal(t, agentID.String(), metadata["agentId"])
	assert.Equal(t, resp.SessionID.String(), metadata["sessionId"])
}

func TestServiceInvokeDoesNotWaitForSlowOperationalAudit(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	auditRecorder := &fakeAuditRecorder{delay: 250 * time.Millisecond}
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})
	_, err := repo.UpsertGrant(context.Background(), "tenant-b", Grant{
		SubjectTenant: "tenant-a",
		AgentID:       agentID,
		Actions:       []string{ActionInvoke},
	})
	require.NoError(t, err)

	executor := newFakeSessionExecutor()
	svc := NewService(repo, agents, executor).WithAuditRecorder(auditRecorder)
	start := time.Now()
	resp, err := svc.Invoke(context.Background(), "tenant-a", InvokeRequest{
		TargetTenant: "tenant-b",
		AgentID:      agentID.String(),
		Input:        "hello",
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	resp.Cancel()
	assert.Less(t, elapsed, 150*time.Millisecond)
	require.Eventually(t, func() bool {
		_, requests := auditRecorder.snapshot()
		return len(requests) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestServiceInvokeUsesIsolatedTargetContextAndCancelsWithSource(t *testing.T) {
	repo := newFakeRepository()
	agents := newFakeAgentReader()
	executor := newFakeSessionExecutor()
	agentID := uuid.New()
	agents.add("tenant-b", AgentRef{ID: agentID, Name: "Target Agent"})
	_, err := repo.UpsertGrant(context.Background(), "tenant-b", Grant{
		SubjectTenant: "tenant-a",
		AgentID:       agentID,
		Actions:       []string{ActionInvoke},
	})
	require.NoError(t, err)

	source, cancelSource := context.WithCancel(tenantctx.NewContextWithToken(context.Background(), "tenant-a", "source-token"))
	defer cancelSource()
	resp, err := NewService(repo, agents, executor).Invoke(source, "tenant-a", InvokeRequest{
		TargetTenant: "tenant-b",
		AgentID:      agentID.String(),
		Input:        "hello",
	})
	require.NoError(t, err)
	defer resp.Cancel()

	assert.Equal(t, "tenant-b", tenantctx.FromContext(executor.createContext))
	assert.Empty(t, tenantctx.TokenFromContext(executor.createContext))
	assert.Equal(t, "tenant-b", tenantctx.FromContext(executor.runContext))
	assert.Empty(t, tenantctx.TokenFromContext(executor.runContext))
	cancelSource()
	require.Eventually(t, func() bool { return executor.runContext.Err() != nil }, time.Second, 10*time.Millisecond)
}
