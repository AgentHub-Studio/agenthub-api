package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentic "github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// BDD scenarios for §5.2 PermissionHandlerPath registry (FEAT-019).
// Source: arXiv:2604.14228v1, Section 5.2 "The Authorization Pipeline".
//
// §5.2 states: "The handler in useCanUseTool.tsx branches into one of four
// paths based on runtime context." The four paths are listed in a numbered
// enumeration (Coordinator, Swarm worker, Speculative classifier, Interactive).
//
// These scenarios validate the observable contract of the typed registry,
// ensuring it accurately mirrors the §5.2 four-path classification and the
// recovery-oriented design intent documented in the paper.

func TestBDD_PermissionHandlerPath(t *testing.T) {
	t.Run("Scenario_FourPathsArePresentInCanonicalAutomationOrder", func(t *testing.T) {
		// Given the §5.2 authorization pipeline describes exactly four handler paths
		// listed from most-automated (Coordinator=1) to least-automated (Interactive=4),
		r := agentic.NewPermissionHandlerPathRegistry()

		// When the registry returns all paths in canonical order,
		all := r.AllPaths()

		// Then exactly four paths exist, ordered by automation level, and the
		// first is Coordinator (most automated) while the last is Interactive
		// (the fallback), matching the §5.2 numbered list exactly.
		require.Len(t, all, 4, "§5.2 enumerates exactly four handler paths")
		assert.Equal(t, agentic.PermissionHandlerPathCoordinator, all[0],
			"path 1 in §5.2 list is Coordinator")
		assert.Equal(t, agentic.PermissionHandlerPathSwarmWorker, all[1],
			"path 2 in §5.2 list is Swarm worker")
		assert.Equal(t, agentic.PermissionHandlerPathSpeculativeClassifier, all[2],
			"path 3 in §5.2 list is Speculative classifier")
		assert.Equal(t, agentic.PermissionHandlerPathInteractive, all[3],
			"path 4 in §5.2 list is Interactive (the fallback)")
	})

	t.Run("Scenario_CoordinatorAndSwarmWorkerAwaitAutomatedChecksBeforeDialog", func(t *testing.T) {
		// Given §5.2 states: "In coordinator and some background paths, automated
		// resolution is attempted before user interaction",
		r := agentic.NewPermissionHandlerPathRegistry()

		// When the registry is asked for background-agent paths,
		bgPaths := r.BackgroundAgentPaths()

		// Then Coordinator and SwarmWorker are both classified as background-agent
		// paths, and both support automated resolution without requiring a user dialog —
		// matching the architectural intent of §5.2 paragraph 4.
		require.Len(t, bgPaths, 2, "two paths are background-agent paths per §5.2")
		pathSet := map[agentic.PermissionHandlerPath]bool{}
		for _, p := range bgPaths {
			pathSet[p] = true
		}
		assert.True(t, pathSet[agentic.PermissionHandlerPathCoordinator],
			"Coordinator must be a background-agent path")
		assert.True(t, pathSet[agentic.PermissionHandlerPathSwarmWorker],
			"SwarmWorker must be a background-agent path")

		coordProf, _ := r.Profile(agentic.PermissionHandlerPathCoordinator)
		swarmProf, _ := r.Profile(agentic.PermissionHandlerPathSwarmWorker)
		assert.False(t, coordProf.RequiresUserDialog,
			"Coordinator must not require a user dialog as its primary mechanism")
		assert.False(t, swarmProf.RequiresUserDialog,
			"SwarmWorker must not require a user dialog as its primary mechanism")
	})

	t.Run("Scenario_SpeculativeClassifierIsGatedByBASH_CLASSIFIERFlag", func(t *testing.T) {
		// Given §5.2 states: "When BASH_CLASSIFIER is enabled and the tool is BashTool,
		// a speculative classifier races a pre-started classification result against a
		// timeout. If the classifier returns with high confidence, the tool is approved
		// instantly without user interaction",
		r := agentic.NewPermissionHandlerPathRegistry()

		// When the registry returns the profile for SpeculativeClassifier,
		prof, ok := r.Profile(agentic.PermissionHandlerPathSpeculativeClassifier)

		// Then the profile correctly records the BASH_CLASSIFIER feature flag,
		// indicating this path is only active when the flag is enabled, and
		// it supports automated resolution (instant approval at high confidence).
		require.True(t, ok)
		assert.Equal(t, "BASH_CLASSIFIER", prof.FeatureFlag,
			"speculative classifier must declare BASH_CLASSIFIER as its feature gate")
		assert.True(t, prof.SupportsAutomatedResolution,
			"speculative classifier can approve without user interaction")
		assert.False(t, prof.RequiresUserDialog,
			"speculative classifier approves instantly at high confidence")
		assert.Equal(t, 3, prof.AutomationLevel,
			"speculative classifier is path 3 in §5.2 list")
	})

	t.Run("Scenario_InteractiveIsAlwaysTheFallbackWithNoFeatureFlag", func(t *testing.T) {
		// Given §5.2 states: "Interactive: The fallback path. Presents the standard
		// user approval dialog through the terminal UI",
		r := agentic.NewPermissionHandlerPathRegistry()

		// When the registry returns the fallback path and its profile,
		fallback := r.FallbackPath()
		prof, ok := r.Profile(fallback)

		// Then the fallback is Interactive, requires the user dialog, does not
		// support automated resolution, and carries no feature flag — it is always
		// available as the last resort, matching §5.2's recovery-oriented design.
		require.True(t, ok)
		assert.Equal(t, agentic.PermissionHandlerPathInteractive, fallback,
			"§5.2: Interactive is the fallback path")
		assert.True(t, prof.RequiresUserDialog,
			"Interactive presents the standard user approval dialog")
		assert.False(t, prof.SupportsAutomatedResolution,
			"Interactive does not attempt automated resolution as the primary mechanism")
		assert.Empty(t, prof.FeatureFlag,
			"Interactive requires no feature flag — it is always available")
		assert.Equal(t, 4, prof.AutomationLevel,
			"Interactive is the least-automated path (level 4)")
	})

	t.Run("Scenario_OnlyOnePathRequiresFeatureFlagSpeculativeClassifier", func(t *testing.T) {
		// Given §5.2 describes three unconditionally-available paths (Coordinator,
		// SwarmWorker, Interactive) and one feature-gated path (SpeculativeClassifier
		// requires BASH_CLASSIFIER to be enabled),
		r := agentic.NewPermissionHandlerPathRegistry()

		// When the registry filters for feature-flag-gated paths,
		flagged := r.PathsRequiringFeatureFlag()

		// Then exactly one path is feature-gated, and it is the SpeculativeClassifier,
		// confirming that the other three paths are always available in their respective
		// runtime contexts.
		require.Len(t, flagged, 1,
			"only SpeculativeClassifier is feature-flag-gated in §5.2")
		assert.Equal(t, agentic.PermissionHandlerPathSpeculativeClassifier, flagged[0],
			"the single gated path is SpeculativeClassifier (BASH_CLASSIFIER flag)")

		// Also verify the other three paths have no feature flag.
		unconditional := []agentic.PermissionHandlerPath{
			agentic.PermissionHandlerPathCoordinator,
			agentic.PermissionHandlerPathSwarmWorker,
			agentic.PermissionHandlerPathInteractive,
		}
		for _, p := range unconditional {
			prof, ok := r.Profile(p)
			require.True(t, ok)
			assert.Empty(t, prof.FeatureFlag,
				"path %q must have no feature flag", p)
		}
	})

	t.Run("Scenario_AutomationLevelsFormStrictAscendingSequence", func(t *testing.T) {
		// Given the §5.2 numbered list implies a strict ordering from most-automated
		// to least-automated (Coordinator=1, SwarmWorker=2, SpeculativeClassifier=3,
		// Interactive=4),
		r := agentic.NewPermissionHandlerPathRegistry()

		// When the canonical order invariant is checked,
		valid := agentic.PermissionHandlerPathAutomationOrderIsAscending()

		// Then the structural invariant holds — automation levels are 1,2,3,4 in order,
		// matching the §5.2 numbered enumeration of the four handler paths.
		assert.True(t, valid,
			"automation levels must form a strictly ascending sequence 1..4 per §5.2")

		// Also verify each individual profile's AutomationLevel is unique in [1,4].
		levelsSeen := map[int]bool{}
		for _, p := range r.AllPaths() {
			prof, ok := r.Profile(p)
			require.True(t, ok)
			assert.False(t, levelsSeen[prof.AutomationLevel],
				"AutomationLevel %d must be unique across all paths", prof.AutomationLevel)
			assert.GreaterOrEqual(t, prof.AutomationLevel, 1)
			assert.LessOrEqual(t, prof.AutomationLevel, 4)
			levelsSeen[prof.AutomationLevel] = true
		}
	})
}
