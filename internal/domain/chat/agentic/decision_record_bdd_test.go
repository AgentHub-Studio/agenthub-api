package agentic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HUMAN-003 — Decision records BDD.
//
// PDF arXiv:2604.14228v1 §11 (decision provenance — agent runtime
// decisions need durable, queryable artifacts so future humans + agents
// understand WHY past choices were made).

func TestBDD_DecisionRecords(t *testing.T) {

	t.Run("Scenario_AgentRecordsRuntimeChoiceWithRationale", func(t *testing.T) {
		// Given an agent picked REST over GraphQL during a run,
		// When it persists a decision record,
		// Then the WHY is captured durably + the choice + the alternatives
		//      considered — future humans can audit the reasoning.
		store := NewInMemoryDecisionRecordStore()
		saved, err := store.Save(context.Background(), DecisionRecord{
			TenantID: "tenant-acme", AgentID: "agent-invoice", RunID: "run-42",
			Title:   "Pick REST over GraphQL for invoice fetch",
			Context: "User asked to fetch the latest invoice",
			Alternatives: []AlternativeOption{
				{Title: "GraphQL", Pros: []string{"flexible queries"}, Cons: []string{"requires schema sync"},
					WhyRejected: "Schema not defined for invoice resource"},
			},
			Choice:    "REST /api/v1/invoices",
			Rationale: "Simpler retries; existing client; aligns with PDF §6.1 simplicity",
			Consequences: []string{
				"Fewer round trips than expected",
				"Won't get partial results",
			},
			Impact: DecisionImpactLocal,
		})
		require.NoError(t, err)
		assert.Equal(t, "tenant-acme", saved.TenantID)
		assert.Equal(t, "REST /api/v1/invoices", saved.Choice)
		assert.NotEmpty(t, saved.Alternatives)
		assert.NotEmpty(t, saved.Consequences)
	})

	t.Run("Scenario_FiveStatusesMirrorADRLifecycle", func(t *testing.T) {
		// Given decision records borrow from ADRs (proposed/accepted/
		//       superseded/rejected/deprecated),
		expected := map[string]bool{
			"proposed": true, "accepted": true, "superseded": true,
			"rejected": true, "deprecated": true,
		}
		for _, s := range AllDecisionStatuses() {
			assert.True(t, expected[string(s)], "status %q not in stable set", s)
		}
		assert.Equal(t, 5, len(AllDecisionStatuses()))
	})

	t.Run("Scenario_FourImpactsClassifyDecisionScope", func(t *testing.T) {
		// Given dashboards aggregate decisions by impact (PDF §11),
		assert.True(t, IsValidDecisionImpact(DecisionImpactLocal))
		assert.True(t, IsValidDecisionImpact(DecisionImpactModule))
		assert.True(t, IsValidDecisionImpact(DecisionImpactSystem))
		assert.True(t, IsValidDecisionImpact(DecisionImpactBusiness))
	})

	t.Run("Scenario_RequiredFieldsEnforceAuditCompleteness", func(t *testing.T) {
		// Given the contract is "no decision without WHO + WHAT + WHY",
		// When required fields are missing, store rejects.
		store := NewInMemoryDecisionRecordStore()
		for _, missing := range []string{"tenantID", "title", "choice", "rationale"} {
			r := DecisionRecord{
				TenantID: "t", Title: "T", Choice: "C", Rationale: "R",
			}
			switch missing {
			case "tenantID":
				r.TenantID = ""
			case "title":
				r.Title = ""
			case "choice":
				r.Choice = ""
			case "rationale":
				r.Rationale = ""
			}
			_, err := store.Save(context.Background(), r)
			assert.Error(t, err, "missing %s must error", missing)
		}
	})

	t.Run("Scenario_DefaultsApplyForOptionalFields", func(t *testing.T) {
		// Given the agent didn't specify status/impact (common case),
		store := NewInMemoryDecisionRecordStore()
		r := DecisionRecord{
			TenantID: "t", Title: "T", Choice: "C", Rationale: "R",
		}
		saved, err := store.Save(context.Background(), r)
		require.NoError(t, err)
		assert.Equal(t, DecisionStatusAccepted, saved.Status, "default = accepted (agent already acted)")
		assert.Equal(t, DecisionImpactLocal, saved.Impact, "default = local")
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantLeak", func(t *testing.T) {
		// Given multi-tenancy is the security boundary,
		store := NewInMemoryDecisionRecordStore()
		r := validRecord()
		r.TenantID = "tenant-A"
		_, _ = store.Save(context.Background(), r)

		// Tenant B asks for the same RunID — must see nothing.
		got, _ := store.ListByRun(context.Background(), "tenant-B", r.RunID)
		assert.Empty(t, got)
	})

	t.Run("Scenario_SupersedeChainsRecordsImmutably", func(t *testing.T) {
		// Given context changed after a decision was made,
		// When the agent records a new decision and supersedes the old,
		// Then the old record is marked superseded with a link forward
		//      — history immutable, navigable.
		store := NewInMemoryDecisionRecordStore()
		old, _ := store.Save(context.Background(), validRecord())
		newR, _ := store.Save(context.Background(), validRecord())
		require.NoError(t, store.Supersede(context.Background(), old.ID, newR.ID))

		got, _ := store.FindByID(context.Background(), old.ID)
		assert.Equal(t, DecisionStatusSuperseded, got.Status)
		require.NotNil(t, got.SupersededBy)
		assert.Equal(t, newR.ID, *got.SupersededBy)
	})

	t.Run("Scenario_FirstSupersedeWinsAuditGuarantee", func(t *testing.T) {
		// Given audit requires "once superseded, the link is immutable",
		store := NewInMemoryDecisionRecordStore()
		old, _ := store.Save(context.Background(), validRecord())
		first, _ := store.Save(context.Background(), validRecord())
		second, _ := store.Save(context.Background(), validRecord())

		require.NoError(t, store.Supersede(context.Background(), old.ID, first.ID))
		err := store.Supersede(context.Background(), old.ID, second.ID)
		assert.True(t, errors.Is(err, ErrDecisionAlreadySuperseded),
			"history rewrite must be REJECTED")
	})

	t.Run("Scenario_HistogramByStatusEnablesDashboards", func(t *testing.T) {
		// Given oncall dashboards show "X accepted / Y rejected over period",
		store := NewInMemoryDecisionRecordStore()
		for i := 0; i < 4; i++ {
			_, _ = store.Save(context.Background(), validRecord())
		}
		r := validRecord()
		r.Status = DecisionStatusRejected
		_, _ = store.Save(context.Background(), r)

		hist, _ := store.CountByStatus(context.Background(), "tenant-x")
		assert.Equal(t, 4, hist[DecisionStatusAccepted])
		assert.Equal(t, 1, hist[DecisionStatusRejected])
		// Stable axes:
		assert.Equal(t, 0, hist[DecisionStatusSuperseded],
			"unused status must appear with 0 — dashboard contract")
	})

	t.Run("Scenario_LongFieldsAreBoundedToPreventLogSpam", func(t *testing.T) {
		// Given LLM-generated rationales / contexts may be long,
		store := NewInMemoryDecisionRecordStore()
		r := validRecord()
		r.Rationale = ""
		for i := 0; i < 2000; i++ {
			r.Rationale += "x"
		}
		saved, _ := store.Save(context.Background(), r)
		assert.Equal(t, 1000, len(saved.Rationale),
			"rationale capped at 1000 chars to keep audit logs sane")
	})

	t.Run("Scenario_ListByRunGroupsDecisionsForRunReview", func(t *testing.T) {
		// Given a reviewer wants to see all decisions made during one run,
		store := NewInMemoryDecisionRecordStore()
		for i := 0; i < 3; i++ {
			_, _ = store.Save(context.Background(), validRecord())
		}
		other := validRecord()
		other.RunID = "different-run"
		_, _ = store.Save(context.Background(), other)

		got, _ := store.ListByRun(context.Background(), "tenant-x", "run-z")
		assert.Len(t, got, 3, "different-run excluded")
	})

	t.Run("Scenario_ListByAgentRespectsLimitForRecentActivity", func(t *testing.T) {
		// Given a UI shows "10 most recent decisions by this agent",
		store := NewInMemoryDecisionRecordStore()
		for i := 0; i < 20; i++ {
			_, _ = store.Save(context.Background(), validRecord())
		}
		got, _ := store.ListByAgent(context.Background(), "tenant-x", "agent-y", 10)
		assert.Len(t, got, 10)
	})

	t.Run("Scenario_AlternativesCarryWhyRejectedForLearning", func(t *testing.T) {
		// Given future agents can learn from past WHY-NOTs,
		store := NewInMemoryDecisionRecordStore()
		r := validRecord()
		r.Alternatives = []AlternativeOption{
			{Title: "Stripe", WhyRejected: "Customer is in EU; SEPA needed"},
			{Title: "Adyen", WhyRejected: "More expensive for low-volume"},
		}
		saved, _ := store.Save(context.Background(), r)
		assert.Len(t, saved.Alternatives, 2)
		for _, alt := range saved.Alternatives {
			assert.NotEmpty(t, alt.WhyRejected,
				"alternatives must explain why-not for future learning")
		}
	})

	t.Run("Scenario_ConsequencesCaptureForeseenOutcomes", func(t *testing.T) {
		// Given consequences (PDF §11 — decision should foresee impact),
		store := NewInMemoryDecisionRecordStore()
		r := validRecord()
		r.Consequences = []string{
			"Lower latency than alternative",
			"No support for streaming",
		}
		saved, _ := store.Save(context.Background(), r)
		assert.Len(t, saved.Consequences, 2)
	})

	t.Run("Scenario_ContextCancellationHonoredAcrossOperations", func(t *testing.T) {
		// Given runner cleanup on cancel,
		store := NewInMemoryDecisionRecordStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.Save(ctx, validRecord())
		assert.Error(t, err)

		_, err = store.FindByID(ctx, uuid.New())
		assert.Error(t, err)
	})
}
