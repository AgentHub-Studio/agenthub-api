package agentic

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validRecord() DecisionRecord {
	return DecisionRecord{
		TenantID:  "tenant-x",
		AgentID:   "agent-y",
		RunID:     "run-z",
		Title:     "Pick REST over GraphQL",
		Context:   "User asked for invoice data",
		Choice:    "REST",
		Rationale: "Simpler retries; existing client",
		Impact:    DecisionImpactLocal,
		Status:    DecisionStatusAccepted,
	}
}

func TestDecision_StatusEnumIsBounded(t *testing.T) {
	for _, s := range AllDecisionStatuses() {
		assert.True(t, IsValidDecisionStatus(s))
	}
	assert.False(t, IsValidDecisionStatus(DecisionStatus("unknown")))
	assert.False(t, IsValidDecisionStatus(""))
}

func TestDecision_AllStatusesCount(t *testing.T) {
	// 5 statuses: proposed/accepted/superseded/rejected/deprecated.
	assert.Equal(t, 5, len(AllDecisionStatuses()))
}

func TestDecision_ImpactEnumIsBounded(t *testing.T) {
	for _, i := range []DecisionImpact{
		DecisionImpactLocal, DecisionImpactModule,
		DecisionImpactSystem, DecisionImpactBusiness,
	} {
		assert.True(t, IsValidDecisionImpact(i))
	}
	assert.False(t, IsValidDecisionImpact(DecisionImpact("global")))
}

func TestDecision_Save_AssignsIDAndTimestamp(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	saved, err := store.Save(context.Background(), validRecord())
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, saved.ID)
	assert.False(t, saved.RecordedAt.IsZero())
}

func TestDecision_Save_RejectsEmptyTenantID(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.TenantID = ""
	_, err := store.Save(context.Background(), r)
	assert.Error(t, err)
}

func TestDecision_Save_RejectsEmptyTitle(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Title = ""
	_, err := store.Save(context.Background(), r)
	assert.Error(t, err)
}

func TestDecision_Save_RejectsEmptyChoice(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Choice = ""
	_, err := store.Save(context.Background(), r)
	assert.Error(t, err)
}

func TestDecision_Save_RejectsEmptyRationale(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Rationale = ""
	_, err := store.Save(context.Background(), r)
	assert.Error(t, err)
}

func TestDecision_Save_DefaultsStatusToAccepted(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Status = ""
	saved, err := store.Save(context.Background(), r)
	require.NoError(t, err)
	assert.Equal(t, DecisionStatusAccepted, saved.Status)
}

func TestDecision_Save_DefaultsImpactToLocal(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Impact = ""
	saved, err := store.Save(context.Background(), r)
	require.NoError(t, err)
	assert.Equal(t, DecisionImpactLocal, saved.Impact)
}

func TestDecision_Save_RejectsInvalidStatus(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Status = DecisionStatus("typo")
	_, err := store.Save(context.Background(), r)
	assert.True(t, errors.Is(err, ErrInvalidDecisionStatus))
}

func TestDecision_Save_RejectsInvalidImpact(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Impact = DecisionImpact("planet")
	_, err := store.Save(context.Background(), r)
	assert.True(t, errors.Is(err, ErrInvalidDecisionImpact))
}

func TestDecision_Save_TruncatesLongFields(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	r := validRecord()
	r.Title = strings.Repeat("x", 500)
	r.Context = strings.Repeat("y", 2000)
	r.Rationale = strings.Repeat("z", 2000)
	saved, err := store.Save(context.Background(), r)
	require.NoError(t, err)
	assert.Equal(t, 200, len(saved.Title))
	assert.Equal(t, 1000, len(saved.Context))
	assert.Equal(t, 1000, len(saved.Rationale))
}

func TestDecision_FindByID_RoundTrips(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	saved, _ := store.Save(context.Background(), validRecord())
	got, err := store.FindByID(context.Background(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.Title, got.Title)
	assert.Equal(t, saved.ID, got.ID)
}

func TestDecision_FindByID_UnknownReturnsErrNotFound(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	_, err := store.FindByID(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrDecisionNotFound))
}

func TestDecision_ListByRun_FiltersAndSortsNewestFirst(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	for i := 0; i < 3; i++ {
		_, _ = store.Save(context.Background(), validRecord())
	}
	r := validRecord()
	r.RunID = "different-run"
	_, _ = store.Save(context.Background(), r)

	got, err := store.ListByRun(context.Background(), "tenant-x", "run-z")
	require.NoError(t, err)
	assert.Len(t, got, 3, "filters by RunID — different-run excluded")
}

func TestDecision_ListByAgent_RespectsLimit(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	for i := 0; i < 5; i++ {
		_, _ = store.Save(context.Background(), validRecord())
	}
	got, err := store.ListByAgent(context.Background(), "tenant-x", "agent-y", 2)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	gotAll, _ := store.ListByAgent(context.Background(), "tenant-x", "agent-y", 0)
	assert.Len(t, gotAll, 5, "limit=0 returns all")
}

func TestDecision_ListByRun_TenantIsolation(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	_, _ = store.Save(context.Background(), validRecord())

	got, err := store.ListByRun(context.Background(), "different-tenant", "run-z")
	require.NoError(t, err)
	assert.Empty(t, got, "tenant isolation enforced")
}

func TestDecision_Supersede_MarksOldAndLinksToNew(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	old, _ := store.Save(context.Background(), validRecord())
	newR, _ := store.Save(context.Background(), validRecord())

	require.NoError(t, store.Supersede(context.Background(), old.ID, newR.ID))

	got, _ := store.FindByID(context.Background(), old.ID)
	assert.Equal(t, DecisionStatusSuperseded, got.Status)
	require.NotNil(t, got.SupersededBy)
	assert.Equal(t, newR.ID, *got.SupersededBy)
}

func TestDecision_Supersede_FirstSupersedeWins(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	old, _ := store.Save(context.Background(), validRecord())
	newR, _ := store.Save(context.Background(), validRecord())
	other, _ := store.Save(context.Background(), validRecord())

	require.NoError(t, store.Supersede(context.Background(), old.ID, newR.ID))
	err := store.Supersede(context.Background(), old.ID, other.ID)
	assert.True(t, errors.Is(err, ErrDecisionAlreadySuperseded))
}

func TestDecision_Supersede_RejectsUnknownNewID(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	old, _ := store.Save(context.Background(), validRecord())
	err := store.Supersede(context.Background(), old.ID, uuid.New())
	assert.Error(t, err)
}

func TestDecision_Supersede_UnknownOldReturnsNotFound(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	newR, _ := store.Save(context.Background(), validRecord())
	err := store.Supersede(context.Background(), uuid.New(), newR.ID)
	assert.True(t, errors.Is(err, ErrDecisionNotFound))
}

func TestDecision_CountByStatus_AlwaysAllStatuses(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	for i := 0; i < 3; i++ {
		_, _ = store.Save(context.Background(), validRecord())
	}
	r := validRecord()
	r.Status = DecisionStatusRejected
	_, _ = store.Save(context.Background(), r)

	hist, err := store.CountByStatus(context.Background(), "tenant-x")
	require.NoError(t, err)
	for _, st := range AllDecisionStatuses() {
		_, ok := hist[st]
		assert.True(t, ok, "status %q must appear in histogram", st)
	}
	assert.Equal(t, 3, hist[DecisionStatusAccepted])
	assert.Equal(t, 1, hist[DecisionStatusRejected])
	assert.Equal(t, 0, hist[DecisionStatusProposed])
}

func TestDecision_ContextCancelled_AllOperationsReturnError(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	saved, _ := store.Save(context.Background(), validRecord())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Save(ctx, validRecord())
	assert.Error(t, err)

	_, err = store.FindByID(ctx, saved.ID)
	assert.Error(t, err)

	_, err = store.ListByRun(ctx, "tenant-x", "run-z")
	assert.Error(t, err)

	_, err = store.ListByAgent(ctx, "tenant-x", "agent-y", 0)
	assert.Error(t, err)

	err = store.Supersede(ctx, saved.ID, saved.ID)
	assert.Error(t, err)

	_, err = store.CountByStatus(ctx, "tenant-x")
	assert.Error(t, err)
}

func TestDecision_ConcurrentSaveIsSafe(t *testing.T) {
	store := NewInMemoryDecisionRecordStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Save(context.Background(), validRecord())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	got, _ := store.ListByAgent(context.Background(), "tenant-x", "agent-y", 0)
	assert.Len(t, got, 50)
}
