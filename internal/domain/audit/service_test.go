package audit_test

import (
	"context"
	"testing"

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

func (m *mockAuditRepo) ListAll(_ context.Context, _ string, _ audit.ListFilter, _ pagination.PageRequest) ([]audit.AuditLog, int, error) {
	out := make([]audit.AuditLog, 0, len(m.data))
	for _, l := range m.data {
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
	m.data[l.ID] = l
	return l, nil
}

const tenantID = "test-tenant"

func TestAuditService_Record_Success(t *testing.T) {
	svc := audit.NewService(newMockRepo())
	log, err := svc.Record(context.Background(), tenantID, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   uuid.New().String(),
		Action:     audit.ActionCreate,
	})
	require.NoError(t, err)
	assert.Equal(t, "agent", log.EntityType)
	assert.NotEqual(t, uuid.Nil, log.ID)
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
			Action:     audit.ActionUpdate,
		})
		require.NoError(t, err)
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, audit.ListFilter{}, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, items, 3)
}
