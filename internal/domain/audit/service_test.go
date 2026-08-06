package audit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockAuditRepo struct {
	data map[uuid.UUID]audit.AuditLog
}

func newMockRepo() *mockAuditRepo {
	return &mockAuditRepo{data: make(map[uuid.UUID]audit.AuditLog)}
}

func (m *mockAuditRepo) ListAll(_ context.Context, _ string, f audit.ListFilter, _ pagination.PageRequest) ([]audit.AuditLog, int, error) {
	out := make([]audit.AuditLog, 0, len(m.data))
	for _, l := range m.data {
		if f.ActorID != "" && l.ActorID != f.ActorID {
			continue
		}
		if f.EntityType != "" && l.EntityType != f.EntityType {
			continue
		}
		if f.Action != "" && string(l.Action) != f.Action {
			continue
		}
		if f.DateFrom != nil && l.CreatedAt.Before(*f.DateFrom) {
			continue
		}
		if f.DateTo != nil && l.CreatedAt.After(*f.DateTo) {
			continue
		}
		out = append(out, l)
	}
	return out, len(out), nil
}

func (m *mockAuditRepo) GetByID(_ context.Context, _ string, id uuid.UUID) (audit.AuditLog, error) {
	l, ok := m.data[id]
	if !ok {
		return audit.AuditLog{}, audit.ErrNotFound
	}
	return l, nil
}

func (m *mockAuditRepo) Record(_ context.Context, _ string, l audit.AuditLog) (audit.AuditLog, error) {
	l.ID = uuid.New()
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	m.data[l.ID] = l
	return l, nil
}

func (m *mockAuditRepo) ApplyRetention(_ context.Context, _ string, cutoff time.Time, dryRun bool) (int, error) {
	deleted := 0
	for id, l := range m.data {
		if l.CreatedAt.Before(cutoff) {
			deleted++
			if !dryRun {
				delete(m.data, id)
			}
		}
	}
	return deleted, nil
}

const tenantID = "test-tenant"

func TestAuditService_Record_Success(t *testing.T) {
	svc := audit.NewService(newMockRepo())
	log, err := svc.Record(context.Background(), tenantID, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   uuid.New().String(),
		Action:     audit.AuditActionCreate,
	})
	require.NoError(t, err)
	assert.Equal(t, "agent", log.EntityType)
	assert.NotEqual(t, uuid.Nil, log.ID)
}

func TestAuditService_Record_WithIPAddress(t *testing.T) {
	svc := audit.NewService(newMockRepo())
	log, err := svc.Record(context.Background(), tenantID, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   uuid.New().String(),
		Action:     audit.AuditActionCreate,
		IPAddress:  "192.168.1.100",
	})
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.100", log.IPAddress)
}

func TestAuditService_GetByID_NotFound(t *testing.T) {
	svc := audit.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, audit.ErrNotFound)
}

func TestAuditService_ListAll(t *testing.T) {
	svc := audit.NewService(newMockRepo())
	for i := 0; i < 3; i++ {
		_, err := svc.Record(context.Background(), tenantID, audit.RecordRequest{
			EntityType: "skill",
			EntityID:   uuid.New().String(),
			Action:     audit.AuditActionUpdate,
		})
		require.NoError(t, err)
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, audit.ListFilter{}, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, items, 3)
}

func TestAuditService_ListAll_FilterByActorID(t *testing.T) {
	svc := audit.NewService(newMockRepo())
	actorA := "actor-a"
	actorB := "actor-b"
	for i := 0; i < 2; i++ {
		_, err := svc.Record(context.Background(), tenantID, audit.RecordRequest{
			EntityType: "agent", EntityID: uuid.New().String(),
			Action: audit.AuditActionCreate, ActorID: actorA,
		})
		require.NoError(t, err)
	}
	_, err := svc.Record(context.Background(), tenantID, audit.RecordRequest{
		EntityType: "agent", EntityID: uuid.New().String(),
		Action: audit.AuditActionCreate, ActorID: actorB,
	})
	require.NoError(t, err)

	items, total, err := svc.ListAll(context.Background(), tenantID,
		audit.ListFilter{ActorID: actorA}, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, items, 2)
}

func TestAuditService_ListAll_FilterByDateRange(t *testing.T) {
	repo := newMockRepo()
	svc := audit.NewService(repo)

	past := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-30 * time.Minute)

	// Inject old entry directly
	old := audit.AuditLog{
		ID: uuid.New(), EntityType: "agent", EntityID: uuid.New().String(),
		Action: audit.AuditActionCreate, CreatedAt: past,
	}
	repo.data[old.ID] = old

	// Record a recent entry
	_, err := svc.Record(context.Background(), tenantID, audit.RecordRequest{
		EntityType: "agent", EntityID: uuid.New().String(), Action: audit.AuditActionUpdate,
	})
	require.NoError(t, err)

	// Filter only last hour
	cutoff := recent
	items, total, err := svc.ListAll(context.Background(), tenantID,
		audit.ListFilter{DateFrom: &cutoff}, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)
}

func TestAuditService_ApplyRetention_DeletesOnlyExpiredRows(t *testing.T) {
	repo := newMockRepo()
	svc := audit.NewService(repo)
	now := time.Date(2026, 6, 23, 8, 0, 0, 0, time.UTC)
	oldID := uuid.New()
	recentID := uuid.New()
	repo.data[oldID] = audit.AuditLog{
		ID: oldID, EntityType: "a2a_invoke", Action: audit.AuditActionExecute,
		CreatedAt: now.AddDate(-2, 0, 0),
	}
	repo.data[recentID] = audit.AuditLog{
		ID: recentID, EntityType: "a2a_invoke", Action: audit.AuditActionExecute,
		CreatedAt: now.AddDate(0, 0, -30),
	}

	resp, err := svc.ApplyRetention(context.Background(), tenantID, audit.AuditRetentionRequest{RetentionDays: 365}, now)

	require.NoError(t, err)
	assert.Equal(t, tenantID, resp.TenantID)
	assert.Equal(t, 365, resp.RetentionDays)
	assert.Equal(t, now.AddDate(0, 0, -365), resp.Cutoff)
	assert.Equal(t, 1, resp.Deleted)
	assert.NotContains(t, repo.data, oldID)
	assert.Contains(t, repo.data, recentID)
}

func TestAuditService_ApplyRetention_DefaultsToPlatformFloorAndSupportsDryRun(t *testing.T) {
	repo := newMockRepo()
	svc := audit.NewService(repo)
	now := time.Date(2026, 6, 23, 8, 0, 0, 0, time.UTC)
	oldID := uuid.New()
	repo.data[oldID] = audit.AuditLog{
		ID: oldID, EntityType: "a2a_invoke", Action: audit.AuditActionExecute,
		CreatedAt: now.AddDate(-2, 0, 0),
	}

	resp, err := svc.ApplyRetention(context.Background(), tenantID, audit.AuditRetentionRequest{DryRun: true}, now)

	require.NoError(t, err)
	assert.Equal(t, audit.DefaultAuditRetentionDays, resp.RetentionDays)
	assert.True(t, resp.DryRun)
	assert.Equal(t, 1, resp.Deleted)
	assert.Contains(t, repo.data, oldID)
}

func TestAuditService_ApplyRetention_RejectsTenantLoweringPlatformFloor(t *testing.T) {
	svc := audit.NewService(newMockRepo())
	_, err := svc.ApplyRetention(context.Background(), tenantID, audit.AuditRetentionRequest{RetentionDays: 30}, time.Now())

	require.ErrorIs(t, err, audit.ErrValidation)
	assert.Contains(t, err.Error(), "retentionDays cannot be lower than 365")
}

// ---- ExtractIP tests ----

func TestExtractIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1")
	assert.Equal(t, "10.0.0.1", audit.ExtractIP(req))
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:12345"
	assert.Equal(t, "203.0.113.5", audit.ExtractIP(req))
}

func TestExtractIP_PreferXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.RemoteAddr = "127.0.0.1:9999"
	assert.Equal(t, "1.2.3.4", audit.ExtractIP(req))
}
