package audit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
)

func TestAuditRetentionScheduler_RunOnceAppliesRetentionToEachTenant(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	lister := &stubRetentionTenantLister{ids: []string{"tenant-a", "tenant-b"}}
	applier := &stubRetentionApplier{deletedByTenant: map[string]int{
		"tenant-a": 2,
		"tenant-b": 3,
	}}
	scheduler := audit.NewRetentionScheduler(lister, applier, audit.RetentionSchedulerConfig{
		Now: func() time.Time { return now },
	})

	result, err := scheduler.RunOnce(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, result.Tenants)
	assert.Equal(t, 5, result.Deleted)
	assert.Equal(t, 0, result.Failed)
	require.Len(t, applier.calls, 2)
	assert.Equal(t, "tenant-a", applier.calls[0].tenantID)
	assert.Equal(t, audit.DefaultAuditRetentionDays, applier.calls[0].req.RetentionDays)
	assert.False(t, applier.calls[0].req.DryRun)
	assert.Equal(t, now, applier.calls[0].now)
	assert.Equal(t, "tenant-b", applier.calls[1].tenantID)
}

func TestAuditRetentionScheduler_RunOnceContinuesWhenTenantFails(t *testing.T) {
	lister := &stubRetentionTenantLister{ids: []string{"tenant-a", "tenant-b", "tenant-c"}}
	applier := &stubRetentionApplier{
		deletedByTenant: map[string]int{"tenant-a": 1, "tenant-c": 4},
		errByTenant:     map[string]error{"tenant-b": errors.New("tenant retention failed")},
	}
	scheduler := audit.NewRetentionScheduler(lister, applier, audit.RetentionSchedulerConfig{})

	result, err := scheduler.RunOnce(context.Background())

	require.Error(t, err)
	assert.Equal(t, 3, result.Tenants)
	assert.Equal(t, 5, result.Deleted)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, applier.calls, 3)
}

func TestAuditRetentionScheduler_ClampsRetentionToPlatformFloor(t *testing.T) {
	lister := &stubRetentionTenantLister{ids: []string{"tenant-a"}}
	applier := &stubRetentionApplier{deletedByTenant: map[string]int{"tenant-a": 1}}
	scheduler := audit.NewRetentionScheduler(lister, applier, audit.RetentionSchedulerConfig{
		RetentionDays: 30,
	})

	_, err := scheduler.RunOnce(context.Background())

	require.NoError(t, err)
	require.Len(t, applier.calls, 1)
	assert.Equal(t, audit.MinAuditRetentionDays, applier.calls[0].req.RetentionDays)
}

type stubRetentionTenantLister struct {
	ids []string
	err error
}

func (s *stubRetentionTenantLister) ListAllIDs(_ context.Context) ([]string, error) {
	return s.ids, s.err
}

type stubRetentionCall struct {
	tenantID string
	req      audit.AuditRetentionRequest
	now      time.Time
}

type stubRetentionApplier struct {
	deletedByTenant map[string]int
	errByTenant     map[string]error
	calls           []stubRetentionCall
}

func (s *stubRetentionApplier) ApplyRetention(_ context.Context, tenantID string, req audit.AuditRetentionRequest, now time.Time) (audit.AuditRetentionResponse, error) {
	s.calls = append(s.calls, stubRetentionCall{tenantID: tenantID, req: req, now: now})
	if err := s.errByTenant[tenantID]; err != nil {
		return audit.AuditRetentionResponse{}, err
	}
	deleted := s.deletedByTenant[tenantID]
	return audit.AuditRetentionResponse{
		TenantID:      tenantID,
		RetentionDays: req.RetentionDays,
		Cutoff:        now.AddDate(0, 0, -req.RetentionDays),
		Deleted:       deleted,
	}, nil
}
