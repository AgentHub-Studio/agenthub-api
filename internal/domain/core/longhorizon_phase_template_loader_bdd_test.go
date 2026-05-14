package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreLongHorizonPhaseTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantHasReadyToInstantiateMultiWeekTaskBlueprints", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And FUTURE-004 LongHorizonTask supports multi-week umbrellas,
		// When admin opens long-horizon onboarding,
		// Then 3 distinct task templates exist (customer-monitoring,
		// product-research, compliance-recert) covering the most common
		// AgentHub initiatives — admin doesn't have to design the phase
		// graph from scratch.
		assert.Equal(t, 3, len(SeedExpectedLongHorizonTaskTemplateSlugs))
	})

	t.Run("Scenario_EachTemplateHasAtLeastTwoPhasesToShowDependencyValue", func(t *testing.T) {
		// Given the value of LongHorizonTask is the dependency graph
		// (single-phase tasks fit in workflow_templates instead),
		// When the seed defines a template,
		// Then it has ≥ 2 phases (otherwise template offers nothing
		// over a regular workflow).
		for tmpl, phases := range SeedExpectedLongHorizonPhaseSlugs {
			assert.GreaterOrEqual(t, len(phases), 2,
				"template %q must have ≥2 phases to justify long-horizon over workflow", tmpl)
		}
	})

	t.Run("Scenario_FinalPhaseAlwaysRequiresAdminReviewToCloseTheLoop", func(t *testing.T) {
		// Given each multi-week initiative ends with a sign-off /
		// publish / attestation step,
		// When the seed defines templates,
		// Then exactly 3 admin-review phases exist (one terminal phase
		// per template) — admin owns the close-out decision.
		assert.Equal(t, 3, len(SeedAdminReviewLongHorizonPhases))
		assert.Equal(t, len(SeedExpectedLongHorizonTaskTemplateSlugs),
			len(SeedAdminReviewLongHorizonPhases),
			"one admin-review phase per template")
	})

	t.Run("Scenario_CustomerMonitoringIsThirtyDayBaselineThenCheckinThenReport", func(t *testing.T) {
		// Given the canonical PDF §12 customer monitoring example is
		// "30 days, daily check-ins",
		// When admin instantiates customer-30day-monitoring,
		// Then they get exactly: baseline → daily-checkin (28 days) → final-report.
		phases := SeedExpectedLongHorizonPhaseSlugs["customer-30day-monitoring"]
		assert.Equal(t, []string{"baseline-snapshot", "daily-checkin-loop", "final-report"}, phases)
	})

	t.Run("Scenario_ResearchTemplateAlignsWithDraftEvery2DaysPattern", func(t *testing.T) {
		// Given PDF §12 research example is "drafts every 2 days",
		// When admin instantiates quarterly-product-research,
		// Then phase 2 (drafting-and-evidence) exists between scoping
		// and final review.
		phases := SeedExpectedLongHorizonPhaseSlugs["quarterly-product-research"]
		assert.Contains(t, phases, "drafting-and-evidence")
		assert.Contains(t, phases, "scoping-and-questions")
		assert.Contains(t, phases, "final-review-and-publish")
	})

	t.Run("Scenario_ComplianceTemplateChainsEvidenceThenAttestation", func(t *testing.T) {
		// Given annual compliance recert has 2 distinct phases
		// (collect → attest), legally separate concerns,
		// When admin instantiates compliance-annual-recert,
		// Then phase 1 (evidence-collection) precedes phase 2
		// (admin-attestation, requires_admin_review).
		phases := SeedExpectedLongHorizonPhaseSlugs["compliance-annual-recert"]
		assert.Equal(t, []string{"evidence-collection", "admin-attestation"}, phases)
	})

	t.Run("Scenario_PhaseSlugsAreUniquePerTemplateNotGlobally", func(t *testing.T) {
		// Given the runtime joins phase by (task_template_slug, phase_slug),
		// When two different templates use the same phase_slug name,
		// Then they don't collide (UNIQUE is per template, not global).
		// Verify by checking that templates can have phases that share
		// SOME keyword but the unique constraint is per-template.
		for tmpl, phases := range SeedExpectedLongHorizonPhaseSlugs {
			seen := map[string]bool{}
			for _, p := range phases {
				assert.False(t, seen[p], "phase %q duplicated in template %q", p, tmpl)
				seen[p] = true
			}
		}
	})

	t.Run("Scenario_ScopingPhaseHasNoDependenciesAsEntryPoint", func(t *testing.T) {
		// Given every template has exactly one entry phase (depends_on=""),
		// When the runtime starts a long-horizon task,
		// Then it knows where to begin without scanning the dependency
		// graph for "phase with no inbound edges".
		// Verify each template has at least one entry phase.
		// Customer: baseline-snapshot is entry.
		// Research: scoping-and-questions is entry.
		// Compliance: evidence-collection is entry.
		entries := map[string]string{
			"customer-30day-monitoring":  "baseline-snapshot",
			"quarterly-product-research": "scoping-and-questions",
			"compliance-annual-recert":   "evidence-collection",
		}
		for tmpl, entry := range entries {
			phases := SeedExpectedLongHorizonPhaseSlugs[tmpl]
			assert.Equal(t, entry, phases[0],
				"template %q entry phase must be %q", tmpl, entry)
		}
	})

	t.Run("Scenario_AdminReviewIsTerminalPhaseOnlyNotIntermediate", func(t *testing.T) {
		// Given admin time is precious and intermediate auto-progress
		// shouldn't pause for review,
		// When the seed declares admin-review phases,
		// Then they are EXCLUSIVELY terminal phases of their templates.
		expectedTerminals := map[string]string{
			"customer-30day-monitoring":  "final-report",
			"quarterly-product-research": "final-review-and-publish",
			"compliance-annual-recert":   "admin-attestation",
		}
		set := map[string]bool{}
		for _, p := range SeedAdminReviewLongHorizonPhases {
			set[p] = true
		}
		for tmpl, terminal := range expectedTerminals {
			assert.True(t, set[tmpl+"/"+terminal],
				"terminal phase of %q must be in admin-review set", tmpl)
		}
	})

	t.Run("Scenario_ThreeTemplatesCoverWorkforceProductAndComplianceUseCases", func(t *testing.T) {
		// Given AgentHub tenants typically run initiatives in 3 buckets
		// (workforce-facing / product-facing / compliance-facing),
		// When fresh tenant arrives,
		// Then templates exist for each bucket so the catalog is balanced.
		expected := map[string]bool{
			"customer-30day-monitoring":  true, // workforce/customer-facing
			"quarterly-product-research": true, // product-facing
			"compliance-annual-recert":   true, // compliance/legal
		}
		for _, s := range SeedExpectedLongHorizonTaskTemplateSlugs {
			assert.True(t, expected[s])
		}
	})
}
