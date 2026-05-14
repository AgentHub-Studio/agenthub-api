package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GOV-005 — Governance report BDD.
//
// PDF arXiv:2604.14228v1 Section 11 (regulator-facing export — auditors
// need machine-parseable dumps); Section 7 (audit-driven reporting feeds
// compliance reviews); CLAUDE.md (audit retention configurable per tenant).
//
// These scenarios validate the unified report contract: aggregation
// across permission/quality/checkpoint/policy sources, flagged-runs
// union with dedup, multi-format export (JSON / CSV / human), tenant
// isolation, period validation, deterministic output.

func TestBDD_GovernanceReport(t *testing.T) {

	t.Run("Scenario_ComplianceOfficerExportsLast24hReport", func(t *testing.T) {
		// Given a compliance officer needs the last 24h of decisions
		//       for tenant 'acme',
		// When the report is built,
		// Then it carries tenant + period + four section summaries.
		rep, err := BuildGovernanceReport("acme", validPeriod(), GovernanceReportInputs{
			Permissions: []ReportPermissionEntry{
				{RunID: "r1", Decision: PermissionAllow},
				{RunID: "r2", Decision: PermissionDeny},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "acme", rep.TenantID)
		assert.True(t, rep.Period.IsValid())
		assert.Equal(t, 1, rep.Permissions.Allow)
		assert.Equal(t, 1, rep.Permissions.Deny)
	})

	t.Run("Scenario_ReportRejectsEmptyTenantToPreventCrossTenantLeak", func(t *testing.T) {
		// Given multi-tenancy is the platform's primary security boundary,
		// When a caller forgets to set tenantID,
		// Then BuildGovernanceReport REJECTS — never produces a global
		//      report that could leak cross-tenant data.
		_, err := BuildGovernanceReport("", validPeriod(), GovernanceReportInputs{})
		assert.Error(t, err)
	})

	t.Run("Scenario_FlaggedRunsUnionAcrossSourcesForReview", func(t *testing.T) {
		// Given a run can be flagged by quality.fail OR checkpoint.rejected
		//       OR policy.deny,
		// When all three sources are present with different RunIDs,
		// Then ALL THREE are flagged (compliance reviews them all).
		in := GovernanceReportInputs{
			Quality:       []ReportQualityEntry{{RunID: "qual-bad", Decision: EvalFail}},
			Checkpoints:   []ReportCheckpointEntry{{RunID: "cp-rej", Decision: CheckpointRejected}},
			PolicyDenials: []ReportPolicyDenialEntry{{RunID: "policy-bad", EngineName: "opa"}},
		}
		rep, _ := BuildGovernanceReport("t", validPeriod(), in)
		assert.Len(t, rep.FlaggedRunIDs, 3)
		assert.Contains(t, rep.FlaggedRunIDs, "qual-bad")
		assert.Contains(t, rep.FlaggedRunIDs, "cp-rej")
		assert.Contains(t, rep.FlaggedRunIDs, "policy-bad")
	})

	t.Run("Scenario_FlaggedRunsDeduplicateSameRunMultipleViolations", func(t *testing.T) {
		// Given a run that fails quality AND has a rejected checkpoint
		//       AND a policy deny — same RunID across sources,
		// When the report flags runs,
		// Then it appears ONCE (no double-counting).
		in := GovernanceReportInputs{
			Quality:       []ReportQualityEntry{{RunID: "really-bad", Decision: EvalFail}},
			Checkpoints:   []ReportCheckpointEntry{{RunID: "really-bad", Decision: CheckpointRejected}},
			PolicyDenials: []ReportPolicyDenialEntry{{RunID: "really-bad", EngineName: "opa"}},
		}
		rep, _ := BuildGovernanceReport("t", validPeriod(), in)
		assert.Len(t, rep.FlaggedRunIDs, 1)
	})

	t.Run("Scenario_NoSilentDataLossWhenAllInputsEmpty", func(t *testing.T) {
		// Given a tenant with zero activity in the period,
		// When the report builds,
		// Then it produces a valid empty report (not nil, not error)
		//      — auditor sees "0 events" not "no data".
		rep, err := BuildGovernanceReport("quiet-tenant", validPeriod(), GovernanceReportInputs{})
		require.NoError(t, err)
		assert.Equal(t, 0, rep.Permissions.Total)
		assert.Equal(t, 0.0, rep.Quality.AverageScore,
			"empty quality average is 0.0 (not panic)")
		assert.False(t, rep.HasFlaggedRuns())
	})

	t.Run("Scenario_PolicyDenialsAttributedPerEngineForAccountability", func(t *testing.T) {
		// Given multiple policy engines may deny — auditor wants to
		//       know WHICH engine was responsible (PDF Section 11
		//       attribution),
		in := GovernanceReportInputs{
			PolicyDenials: []ReportPolicyDenialEntry{
				{RunID: "r1", EngineName: "enterprise-opa"},
				{RunID: "r2", EngineName: "enterprise-opa"},
				{RunID: "r3", EngineName: "tenant-rules"},
			},
		}
		rep, _ := BuildGovernanceReport("t", validPeriod(), in)
		assert.Equal(t, 2, rep.PolicyDenials.ByEngine["enterprise-opa"])
		assert.Equal(t, 1, rep.PolicyDenials.ByEngine["tenant-rules"])
		assert.Equal(t, 3, rep.PolicyDenials.Total)
	})

	t.Run("Scenario_CSVExportIsRegulatorReady", func(t *testing.T) {
		// Given regulators commonly require CSV (or PDF/XLSX),
		// When CSV is rendered,
		// Then header + section/metric/value rows produce a flat,
		//      grep-friendly format.
		rep, _ := BuildGovernanceReport("acme", validPeriod(), GovernanceReportInputs{
			Permissions: []ReportPermissionEntry{
				{RunID: "r1", Decision: PermissionAllow},
				{RunID: "r2", Decision: PermissionDeny},
			},
		})
		csv, err := rep.CSV()
		require.NoError(t, err)
		assert.Contains(t, csv, "section,metric,value")
		assert.Contains(t, csv, "meta,tenantId,acme")
		assert.Contains(t, csv, "permissions,allow,1")
		assert.Contains(t, csv, "permissions,deny,1")
	})

	t.Run("Scenario_CSVOrderingIsDeterministicForByteStableAudit", func(t *testing.T) {
		// Given regulators byte-compare CSV exports across periods,
		//       per-engine rows MUST sort deterministically (Go map
		//       iteration is randomized — sorting is mandatory).
		in := GovernanceReportInputs{
			PolicyDenials: []ReportPolicyDenialEntry{
				{RunID: "r1", EngineName: "z-engine"},
				{RunID: "r2", EngineName: "a-engine"},
			},
		}
		rep, _ := BuildGovernanceReport("t", validPeriod(), in)
		csv, _ := rep.CSV()
		aIdx := strings.Index(csv, "policyDenials.byEngine,a-engine")
		zIdx := strings.Index(csv, "policyDenials.byEngine,z-engine")
		assert.Less(t, aIdx, zIdx, "a-engine sorted before z-engine")
	})

	t.Run("Scenario_HumanSummaryDriversAlertingOnFlaggedRuns", func(t *testing.T) {
		// Given an oncall channel posts the summary to operators,
		// When there are flagged runs,
		// Then the summary leads with "FLAGGED FOR REVIEW: N runs"
		//      so the eye finds it without scanning columns.
		in := GovernanceReportInputs{
			Quality: []ReportQualityEntry{
				{RunID: "run-001", Decision: EvalFail},
				{RunID: "run-002", Decision: EvalFail},
			},
		}
		rep, _ := BuildGovernanceReport("t", validPeriod(), in)
		summary := rep.HumanSummary()
		assert.Contains(t, summary, "FLAGGED FOR REVIEW: 2")
		assert.Contains(t, summary, "- run-001")
		assert.Contains(t, summary, "- run-002")
	})

	t.Run("Scenario_HumanSummaryStatesNoFlagsExplicitly", func(t *testing.T) {
		// Given silence is dangerous (auditor wonders "did the report
		//       even check?"),
		// When zero flagged runs,
		// Then the summary EXPLICITLY says "No runs flagged".
		rep, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{})
		summary := rep.HumanSummary()
		assert.Contains(t, summary, "No runs flagged",
			"explicit zero is mandatory — silence breeds doubt")
	})

	t.Run("Scenario_PeriodValidationCatchesInvertedTimeWindow", func(t *testing.T) {
		// Given inverted period (end < start) is a caller bug,
		// When the report builds,
		// Then it returns ErrInvalidReportPeriod — fail fast, not
		//      silently produce nonsense.
		_, err := BuildGovernanceReport("t", ReportPeriod{}, GovernanceReportInputs{})
		assert.Error(t, err)
	})

	t.Run("Scenario_AverageScoreIsExplicitForEmptyQuality", func(t *testing.T) {
		// Given empty quality entries,
		// When average is computed,
		// Then the result is 0.0 — never NaN, never panic from div-by-zero.
		rep, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{
			Quality: nil,
		})
		assert.Equal(t, 0.0, rep.Quality.AverageScore)
	})

	t.Run("Scenario_JSONExportCarriesAllSectionsForAPIConsumers", func(t *testing.T) {
		// Given API consumers parse JSON for dashboards,
		// When JSON is rendered,
		// Then it carries all 4 sections + meta + flagged list.
		rep, _ := BuildGovernanceReport("acme", validPeriod(), GovernanceReportInputs{
			Permissions: []ReportPermissionEntry{{RunID: "r", Decision: PermissionAllow}},
		})
		jsonStr, err := rep.JSON()
		require.NoError(t, err)
		assert.Contains(t, jsonStr, `"tenantId": "acme"`)
		assert.Contains(t, jsonStr, `"period"`)
		assert.Contains(t, jsonStr, `"permissions"`)
		assert.Contains(t, jsonStr, `"quality"`)
		assert.Contains(t, jsonStr, `"checkpoints"`)
		assert.Contains(t, jsonStr, `"policyDenials"`)
	})

	t.Run("Scenario_AlertingHookSeesHasFlaggedRunsBoolean", func(t *testing.T) {
		// Given a webhook integration polls reports and triggers PagerDuty
		//       when flags exist,
		// When the report is queried,
		// Then HasFlaggedRuns gives a single bool — no need to count
		//      array length in caller code.
		flagged, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{
			Quality: []ReportQualityEntry{{RunID: "x", Decision: EvalFail}},
		})
		assert.True(t, flagged.HasFlaggedRuns())

		clean, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{
			Quality: []ReportQualityEntry{{RunID: "x", Decision: EvalPass}},
		})
		assert.False(t, clean.HasFlaggedRuns())
	})
}
