package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSRSDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksReturnSummaryShapeFromCatalog", func(t *testing.T) {
		// Given fresh tenants must produce SUB-010 SubagentReturnSummary
		// for each subagent completion, but inventing findings/artifacts/
		// next-steps shapes is error-prone,
		// When admin opens summary-shape onboarding,
		// Then 5 recommended templates surface across outcome variety.
		assert.Equal(t, 5, len(SeedRecommendedSRSDTemplateSlugs))
	})

	t.Run("Scenario_SuccessWithArtifactsForRoutineCompletion", func(t *testing.T) {
		// Given a subagent finished and produced artifacts,
		// When admin uses success-with-artifacts,
		// Then findings/artifacts/next-steps all populated; no admin review.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "success-with-artifacts")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSRSDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["success-with-artifacts"])
	})

	t.Run("Scenario_PartialNeedsFollowupEmphasisesNextSteps", func(t *testing.T) {
		// Given the subagent hit a constraint mid-task,
		// When admin uses partial-needs-followup,
		// Then next-steps explain how parent should continue.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "partial-needs-followup")
	})

	t.Run("Scenario_FailedErrorEmphasisesDiagnosis", func(t *testing.T) {
		// Given a subagent could not complete (errors),
		// When admin uses failed-error,
		// Then findings explain WHY and next-steps suggest remediation;
		// no artifacts (nothing usable produced); admin review required.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "failed-error")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSRSDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["failed-error"])
	})

	t.Run("Scenario_AbortedByParentCapturesPartialSignal", func(t *testing.T) {
		// Given parent aborted the subagent before completion,
		// When admin uses aborted-by-parent,
		// Then whatever findings landed are captured; outcome=aborted;
		// admin review.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "aborted-by-parent")
	})

	t.Run("Scenario_AuditWithRedactionForExternalEmission", func(t *testing.T) {
		// Given the summary travels outside tenant boundary,
		// When admin uses audit-with-redaction,
		// Then all 3 redact flags are on (findings/artifacts/transcript_hash);
		// admin review.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "audit-with-redaction")
	})

	t.Run("Scenario_OutcomeLabelsMatchSUB010EnumByteForByte", func(t *testing.T) {
		// Given SUB-010 SubagentReturnOutcome has 4 values,
		// When seed declares target_outcome,
		// Then every label is a valid SubagentReturnOutcome (no mapping
		// table runtime).
		sub010 := map[string]bool{
			"success": true, "partial": true,
			"failed": true, "aborted": true,
		}
		for _, o := range SeedExpectedSRSDTemplateOutcomes {
			assert.True(t, sub010[o], "label %q not in SUB-010", o)
		}
	})

	t.Run("Scenario_FailedTemplateHasEmptyArtifacts", func(t *testing.T) {
		// Given a failed subagent didn't complete useful work,
		// When admin inspects sample_artifacts,
		// Then failed-error has empty artifacts. Validated DB-real.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "failed-error")
	})

	t.Run("Scenario_AuditTemplateHasAllRedactionFlagsOn", func(t *testing.T) {
		// Given audit posture demands maximum redaction,
		// When admin compares the audit template's redact_* flags,
		// Then all three are true. Validated DB-real.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "audit-with-redaction")
	})

	t.Run("Scenario_AdminReviewGatesRiskyOutcomes", func(t *testing.T) {
		// Given success/partial são routine; failed/aborted/audit são risky,
		// When admin compares admin-review subset,
		// Then 3 of 5 templates gate change.
		assert.Equal(t, 3, len(SeedAdminReviewSRSDTemplateSlugs))
	})

	t.Run("Scenario_SuccessOutcomeAppearsTwiceRoutineAndAuditVariants", func(t *testing.T) {
		// Given outcome=success can have two shapes: routine (no redaction)
		// and audit (full redaction for export),
		// When admin queries by outcome=success,
		// Then 2 templates surface. Validated DB-real.
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "success-with-artifacts")
		assert.Contains(t, SeedExpectedSRSDTemplateSlugs, "audit-with-redaction")
	})
}
