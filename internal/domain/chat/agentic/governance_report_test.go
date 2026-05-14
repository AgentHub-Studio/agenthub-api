package agentic

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validPeriod() ReportPeriod {
	now := time.Now().UTC()
	return ReportPeriod{Start: now.Add(-24 * time.Hour), End: now}
}

func TestGovReport_Period_IsValid(t *testing.T) {
	assert.True(t, validPeriod().IsValid())
	assert.False(t, ReportPeriod{}.IsValid())
	assert.False(t, ReportPeriod{Start: time.Now(), End: time.Time{}}.IsValid())
	now := time.Now()
	assert.False(t, ReportPeriod{Start: now, End: now.Add(-time.Hour)}.IsValid(),
		"end < start invalid")
}

func TestGovReport_Build_RejectsEmptyTenant(t *testing.T) {
	_, err := BuildGovernanceReport("", validPeriod(), GovernanceReportInputs{})
	assert.True(t, errors.Is(err, ErrEmptyTenantID))
}

func TestGovReport_Build_RejectsInvalidPeriod(t *testing.T) {
	_, err := BuildGovernanceReport("tenant-x", ReportPeriod{}, GovernanceReportInputs{})
	assert.True(t, errors.Is(err, ErrInvalidReportPeriod))
}

func TestGovReport_Build_AggregatesPermissions(t *testing.T) {
	in := GovernanceReportInputs{
		Permissions: []ReportPermissionEntry{
			{RunID: "r1", Decision: PermissionAllow},
			{RunID: "r2", Decision: PermissionAllow},
			{RunID: "r3", Decision: PermissionDeny},
			{RunID: "r4", Decision: PermissionConfirm},
		},
	}
	rep, err := BuildGovernanceReport("t", validPeriod(), in)
	require.NoError(t, err)
	assert.Equal(t, 2, rep.Permissions.Allow)
	assert.Equal(t, 1, rep.Permissions.Deny)
	assert.Equal(t, 1, rep.Permissions.Confirm)
	assert.Equal(t, 4, rep.Permissions.Total)
}

func TestGovReport_Build_AggregatesQualityWithAverage(t *testing.T) {
	in := GovernanceReportInputs{
		Quality: []ReportQualityEntry{
			{RunID: "r1", OverallScore: 0.9, Decision: EvalPass},
			{RunID: "r2", OverallScore: 0.6, Decision: EvalWarn},
			{RunID: "r3", OverallScore: 0.3, Decision: EvalFail},
		},
	}
	rep, err := BuildGovernanceReport("t", validPeriod(), in)
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Quality.Pass)
	assert.Equal(t, 1, rep.Quality.Warn)
	assert.Equal(t, 1, rep.Quality.Fail)
	assert.Equal(t, 3, rep.Quality.Total)
	assert.InDelta(t, (0.9+0.6+0.3)/3, rep.Quality.AverageScore, 0.0001)
}

func TestGovReport_Build_EmptyQualityProducesZeroAverageNotDivByZero(t *testing.T) {
	rep, err := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{})
	require.NoError(t, err)
	assert.Equal(t, 0.0, rep.Quality.AverageScore,
		"empty quality must produce 0.0 not panic")
}

func TestGovReport_Build_AggregatesCheckpoints(t *testing.T) {
	in := GovernanceReportInputs{
		Checkpoints: []ReportCheckpointEntry{
			{RunID: "r1", Decision: CheckpointApproved},
			{RunID: "r2", Decision: CheckpointApproved},
			{RunID: "r3", Decision: CheckpointRejected},
			{RunID: "r4", Decision: CheckpointTimedOut},
			{RunID: "r5", Decision: CheckpointCancelled},
		},
	}
	rep, err := BuildGovernanceReport("t", validPeriod(), in)
	require.NoError(t, err)
	assert.Equal(t, 2, rep.Checkpoints.Approved)
	assert.Equal(t, 1, rep.Checkpoints.Rejected)
	assert.Equal(t, 1, rep.Checkpoints.TimedOut)
	assert.Equal(t, 1, rep.Checkpoints.Cancelled)
	assert.Equal(t, 5, rep.Checkpoints.Total)
}

func TestGovReport_Build_AggregatesPolicyDenialsByEngine(t *testing.T) {
	in := GovernanceReportInputs{
		PolicyDenials: []ReportPolicyDenialEntry{
			{RunID: "r1", EngineName: "opa"},
			{RunID: "r2", EngineName: "opa"},
			{RunID: "r3", EngineName: "policy-limits"},
		},
	}
	rep, err := BuildGovernanceReport("t", validPeriod(), in)
	require.NoError(t, err)
	assert.Equal(t, 3, rep.PolicyDenials.Total)
	assert.Equal(t, 2, rep.PolicyDenials.ByEngine["opa"])
	assert.Equal(t, 1, rep.PolicyDenials.ByEngine["policy-limits"])
}

func TestGovReport_Build_FlaggedRunsUnionAcrossSources(t *testing.T) {
	in := GovernanceReportInputs{
		Quality: []ReportQualityEntry{
			{RunID: "qual-fail-1", OverallScore: 0.2, Decision: EvalFail},
		},
		Checkpoints: []ReportCheckpointEntry{
			{RunID: "checkpoint-rej-1", Decision: CheckpointRejected},
		},
		PolicyDenials: []ReportPolicyDenialEntry{
			{RunID: "policy-deny-1", EngineName: "opa"},
		},
	}
	rep, err := BuildGovernanceReport("t", validPeriod(), in)
	require.NoError(t, err)
	require.Len(t, rep.FlaggedRunIDs, 3)
	// Sorted ascending.
	assert.Equal(t, []string{"checkpoint-rej-1", "policy-deny-1", "qual-fail-1"},
		rep.FlaggedRunIDs)
}

func TestGovReport_Build_FlaggedRunsDeduplicateAcrossSources(t *testing.T) {
	in := GovernanceReportInputs{
		Quality: []ReportQualityEntry{
			{RunID: "shared-bad", OverallScore: 0.1, Decision: EvalFail},
		},
		Checkpoints: []ReportCheckpointEntry{
			{RunID: "shared-bad", Decision: CheckpointRejected},
		},
		PolicyDenials: []ReportPolicyDenialEntry{
			{RunID: "shared-bad", EngineName: "opa"},
		},
	}
	rep, err := BuildGovernanceReport("t", validPeriod(), in)
	require.NoError(t, err)
	require.Len(t, rep.FlaggedRunIDs, 1, "same RunID across sources counted once")
	assert.Equal(t, "shared-bad", rep.FlaggedRunIDs[0])
}

func TestGovReport_Build_PassDoesNotFlag(t *testing.T) {
	in := GovernanceReportInputs{
		Quality: []ReportQualityEntry{
			{RunID: "good", OverallScore: 0.9, Decision: EvalPass},
		},
		Checkpoints: []ReportCheckpointEntry{
			{RunID: "approved", Decision: CheckpointApproved},
		},
	}
	rep, err := BuildGovernanceReport("t", validPeriod(), in)
	require.NoError(t, err)
	assert.Empty(t, rep.FlaggedRunIDs)
	assert.False(t, rep.HasFlaggedRuns())
}

func TestGovReport_JSON_RoundTrip(t *testing.T) {
	rep, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{
		Permissions: []ReportPermissionEntry{{RunID: "r", Decision: PermissionAllow}},
	})
	jsonStr, err := rep.JSON()
	require.NoError(t, err)

	var roundtrip GovernanceReport
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &roundtrip))
	assert.Equal(t, rep.TenantID, roundtrip.TenantID)
	assert.Equal(t, rep.Permissions.Allow, roundtrip.Permissions.Allow)
}

func TestGovReport_CSV_HasMetaSectionAndStableOrder(t *testing.T) {
	rep, _ := BuildGovernanceReport("tenant-x", validPeriod(), GovernanceReportInputs{
		Permissions: []ReportPermissionEntry{{RunID: "r", Decision: PermissionAllow}},
	})
	csvStr, err := rep.CSV()
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(csvStr), "\n")
	assert.Equal(t, "section,metric,value", lines[0])
	assert.Contains(t, lines[1], "meta,tenantId,tenant-x")
	assert.Contains(t, csvStr, "permissions,allow,1")
}

func TestGovReport_CSV_PolicyDenialsSortedDeterministically(t *testing.T) {
	rep, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{
		PolicyDenials: []ReportPolicyDenialEntry{
			{RunID: "r1", EngineName: "zeta"},
			{RunID: "r2", EngineName: "alpha"},
			{RunID: "r3", EngineName: "alpha"},
		},
	})
	csvStr, err := rep.CSV()
	require.NoError(t, err)
	// alpha appears before zeta in CSV (sorted).
	alphaIdx := strings.Index(csvStr, "policyDenials.byEngine,alpha")
	zetaIdx := strings.Index(csvStr, "policyDenials.byEngine,zeta")
	assert.Greater(t, alphaIdx, 0)
	assert.Greater(t, zetaIdx, 0)
	assert.Less(t, alphaIdx, zetaIdx, "alpha sorted before zeta")
}

func TestGovReport_CSV_DeterministicOnSameInput(t *testing.T) {
	in := GovernanceReportInputs{
		Permissions: []ReportPermissionEntry{
			{RunID: "r1", Decision: PermissionAllow},
		},
		Quality: []ReportQualityEntry{
			{RunID: "r2", OverallScore: 0.5, Decision: EvalWarn},
		},
	}
	period := validPeriod()
	rep1, _ := BuildGovernanceReport("t", period, in)
	rep2, _ := BuildGovernanceReport("t", period, in)
	// Force GeneratedAt equal so byte-comparison is stable.
	rep2.GeneratedAt = rep1.GeneratedAt
	csv1, _ := rep1.CSV()
	csv2, _ := rep2.CSV()
	assert.Equal(t, csv1, csv2,
		"same input → same CSV (byte-stable for regulator dumps)")
}

func TestGovReport_HumanSummary_ContainsAllSections(t *testing.T) {
	rep, _ := BuildGovernanceReport("tenant-y", validPeriod(), GovernanceReportInputs{
		Permissions: []ReportPermissionEntry{{RunID: "r", Decision: PermissionAllow}},
		Quality:     []ReportQualityEntry{{RunID: "f", OverallScore: 0.1, Decision: EvalFail}},
	})
	summary := rep.HumanSummary()
	assert.Contains(t, summary, "tenant-y")
	assert.Contains(t, summary, "Permissions:")
	assert.Contains(t, summary, "Quality:")
	assert.Contains(t, summary, "Checkpoints:")
	assert.Contains(t, summary, "Policy denials:")
	assert.Contains(t, summary, "FLAGGED FOR REVIEW: 1")
	assert.Contains(t, summary, "- f")
}

func TestGovReport_HumanSummary_NoFlagsMessage(t *testing.T) {
	rep, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{})
	summary := rep.HumanSummary()
	assert.Contains(t, summary, "No runs flagged")
}

func TestGovReport_HasFlaggedRuns(t *testing.T) {
	noFlag, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{})
	assert.False(t, noFlag.HasFlaggedRuns())

	flag, _ := BuildGovernanceReport("t", validPeriod(), GovernanceReportInputs{
		Quality: []ReportQualityEntry{{RunID: "x", OverallScore: 0.1, Decision: EvalFail}},
	})
	assert.True(t, flag.HasFlaggedRuns())
}
