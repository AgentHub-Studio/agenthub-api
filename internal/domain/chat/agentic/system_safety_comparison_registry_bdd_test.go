package agentic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// system_safety_comparison_registry_bdd_test.go — BDD tests for FEAT-047
//
// Scenarios derived from §13.2 "Safety and permissions" paragraph and §5.1:
//
//   Scenario 1: Registry identifies exactly four §13.2 systems
//   Scenario 2: Claude Code is the only deny-first human-in-loop system
//   Scenario 3: SWE-Agent and OpenHands share Docker isolation safety model
//   Scenario 4: Aider uses Git rollback as sole safety mechanism
//   Scenario 5: Approval-model comparison table covers all four systems
//   Scenario 6: All structural invariants hold simultaneously
//   Scenario 7: Each system belongs to exactly one isolation-boundary category
//   Scenario 8: ML classifier uniquely distinguishes Claude Code from the field

// Scenario 1: Registry identifies exactly four §13.2 systems
// Given the paper §13.2 names four systems with distinct safety profiles
// When the registry is constructed
// Then Count() == 4, AllSystems() returns 4 items, and each has a non-empty PDFEvidence
func TestBDD_Scenario1_RegistryIdentifiesExactlyFourSystems(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()

	// When
	count := r.Count()
	systems := r.AllSystems()

	// Then
	assert.Equal(t, 4, count, "Count() must equal 4 per §13.2")
	require.Len(t, systems, 4, "AllSystems() must return exactly 4 profiles")

	for _, p := range systems {
		assert.NotEmpty(t, p.SystemID, "every system must have a non-empty SystemID")
		assert.NotEmpty(t, p.SystemName, "every system must have a non-empty SystemName")
		assert.NotEmpty(t, p.PDFEvidence, "every system must have PDFEvidence from §13.2")
		assert.Equal(t, "13.2", p.PDFSection, "all systems are sourced from §13.2")
	}
}

// Scenario 2: Claude Code is the only deny-first human-in-loop system
// Given §5.1 documents that Claude Code uses deny-first evaluation with user escalation
// When filtering by human-in-loop and by defaults-to-safe
// Then exactly Claude Code appears in both filters; all others do not
func TestBDD_Scenario2_ClaudeCodeOnlyDenyFirstHumanInLoopSystem(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()
	allOtherSystems := []AgentSystemID{SafetySystemSWEAgent, SafetySystemOpenHands, SafetySystemAider}

	// When — human-in-loop filter
	humanLoop := r.SystemsWithHumanInLoop()

	// Then
	require.Len(t, humanLoop, 1, "exactly one system should be human-in-loop")
	assert.Equal(t, SafetySystemClaudeCode, humanLoop[0].SystemID)
	assert.Equal(t, ApprovalPerActionDenyFirst, humanLoop[0].ApprovalModel,
		"Claude Code's human-in-loop approval model must be per-action deny-first")

	// When — defaults-to-safe filter
	safe := r.SystemsDefaultingSafe()

	// Then
	require.Len(t, safe, 1, "exactly one system should default to safe")
	assert.Equal(t, SafetySystemClaudeCode, safe[0].SystemID)

	// And — none of the other systems are human-in-loop or default-safe
	for _, id := range allOtherSystems {
		p, _ := r.FindByID(id)
		assert.False(t, p.IsHumanInLoop, "system %s must not be human-in-loop", id)
		assert.False(t, p.DefaultsToSafe, "system %s must not default to safe", id)
	}
}

// Scenario 3: SWE-Agent and OpenHands share Docker isolation safety model
// Given §13.2: "SWE-Agent and OpenHands rely primarily on Docker container isolation"
// When their profiles are compared across all three safety axes
// Then they are identical on ApprovalModel, IsolationBoundary, and RecoveryMechanism
func TestBDD_Scenario3_SWEAgentAndOpenHandsShareDockerSafetyModel(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()

	// When
	swe, sweOK := r.FindByID(SafetySystemSWEAgent)
	oh, ohOK := r.FindByID(SafetySystemOpenHands)
	containerSystems := r.SystemsWithContainerIsolation()

	// Then — both found
	require.True(t, sweOK)
	require.True(t, ohOK)

	// And — identical safety axes per §13.2
	assert.Equal(t, swe.ApprovalModel, oh.ApprovalModel,
		"SWE-Agent and OpenHands must share the same approval model")
	assert.Equal(t, swe.IsolationBoundary, oh.IsolationBoundary,
		"SWE-Agent and OpenHands must share the same isolation boundary")
	assert.Equal(t, swe.RecoveryMechanism, oh.RecoveryMechanism,
		"SWE-Agent and OpenHands must share the same recovery mechanism")

	// And — both use Docker container
	assert.Equal(t, IsolationDockerContainer, swe.IsolationBoundary)
	assert.Equal(t, IsolationDockerContainer, oh.IsolationBoundary)
	assert.True(t, swe.UsesContainerIsolation)
	assert.True(t, oh.UsesContainerIsolation)

	// And — the container-isolation filter returns exactly these two
	assert.Len(t, containerSystems, 2)
	containerIDs := make(map[AgentSystemID]bool)
	for _, cs := range containerSystems {
		containerIDs[cs.SystemID] = true
	}
	assert.True(t, containerIDs[SafetySystemSWEAgent])
	assert.True(t, containerIDs[SafetySystemOpenHands])
}

// Scenario 4: Aider uses Git rollback as sole safety mechanism
// Given §13.2: "Aider uses Git as its primary safety mechanism"
// When the Aider profile is examined across all safety axes
// Then IsolationBoundary==none, ApprovalModel==version_control_as_net, RecoveryMechanism==git_rollback
func TestBDD_Scenario4_AiderUsesGitRollbackAsSoleSafetyMechanism(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()

	// When
	aider, ok := r.FindByID(SafetySystemAider)
	gitSystems := r.SystemsWithGitRollback()

	// Then
	require.True(t, ok)
	assert.Equal(t, ApprovalVersionControlAsNet, aider.ApprovalModel,
		"Aider's approval model must be version_control_as_net per §13.2")
	assert.Equal(t, IsolationNone, aider.IsolationBoundary,
		"Aider has no dedicated runtime isolation boundary per §13.2")
	assert.Equal(t, RecoveryGitRollback, aider.RecoveryMechanism,
		"Aider's recovery mechanism must be git_rollback per §13.2")
	assert.True(t, aider.SupportsGitRollback,
		"Aider.SupportsGitRollback must be true per §13.2")
	assert.False(t, aider.IsHumanInLoop,
		"Aider does not use interactive human approval in its safety flow")
	assert.False(t, aider.UsesContainerIsolation,
		"Aider does not use container isolation")

	// And — git rollback uniquely identifies Aider among the four systems
	require.Len(t, gitSystems, 1,
		"exactly one §13.2 system uses git rollback as primary safety mechanism")
	assert.Equal(t, SafetySystemAider, gitSystems[0].SystemID)
}

// Scenario 5: Approval-model comparison table covers all four systems
// Given §13.2 discusses approval models for all four systems
// When CompareApprovalModels() is called
// Then the result contains four distinct entries, one per system
// And each entry is a "SystemName: approval_model" formatted string
func TestBDD_Scenario5_ApprovalModelComparisonTableCoversFourSystems(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()
	expectedSystems := []string{"Claude Code", "SWE-Agent", "OpenHands", "Aider"}
	expectedModels := []string{
		string(ApprovalPerActionDenyFirst),
		string(ApprovalContainerBoundary),
		string(ApprovalContainerBoundary),
		string(ApprovalVersionControlAsNet),
	}

	// When
	entries := r.CompareApprovalModels()

	// Then
	require.Len(t, entries, 4, "comparison table must have exactly 4 entries")
	for i, entry := range entries {
		assert.True(t, strings.Contains(entry, expectedSystems[i]),
			"entry %d must contain system name %q, got %q", i, expectedSystems[i], entry)
		assert.True(t, strings.Contains(entry, expectedModels[i]),
			"entry %d must contain approval model %q, got %q", i, expectedModels[i], entry)
		assert.True(t, strings.Contains(entry, ": "),
			"entry %d must use 'SystemName: model' format, got %q", i, entry)
	}

	// And — entries are in canonical §13.2 order
	assert.Equal(t, fmt.Sprintf("Claude Code: %s", ApprovalPerActionDenyFirst), entries[0])
	assert.Equal(t, fmt.Sprintf("Aider: %s", ApprovalVersionControlAsNet), entries[3])
}

// Scenario 6: All structural invariants hold simultaneously
// Given §13.2 describes a coherent four-system safety landscape
// When StructuralInvariants() is evaluated
// Then all four named invariants hold true
func TestBDD_Scenario6_AllStructuralInvariantsHoldSimultaneously(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()
	expectedInvariants := []string{
		"ExactlyFourSystems",
		"ClaudeCodeIsHumanInLoop",
		"GitRollbackSystemsAreSafe",
		"ContainerSystemsNotDenyFirst",
	}

	// When
	results := r.StructuralInvariants()

	// Then
	require.Len(t, results, 4, "must have exactly 4 structural invariants")

	resultMap := make(map[string]SafetyInvariantResult)
	for _, inv := range results {
		resultMap[inv.Name] = inv
	}

	for _, name := range expectedInvariants {
		inv, found := resultMap[name]
		require.True(t, found, "invariant %q must be present", name)
		assert.True(t, inv.Holds, "invariant %q must hold: %s", name, inv.Message)
	}
}

// Scenario 7: Each system belongs to exactly one isolation-boundary category
// Given §13.2 assigns a distinct isolation strategy to each system (or pair)
// When all isolation boundary filters are evaluated
// Then the union of results equals all four systems with no overlaps
func TestBDD_Scenario7_EachSystemBelongsToExactlyOneIsolationCategory(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()
	allBoundaries := []IsolationBoundary{
		IsolationLayeredPolicyEnforcement,
		IsolationDockerContainer,
		IsolationSandboxModes,
		IsolationNone,
	}

	// When — collect all systems from all isolation categories
	seen := make(map[AgentSystemID]int)
	for _, b := range allBoundaries {
		for _, p := range r.SystemsByIsolationBoundary(b) {
			seen[p.SystemID]++
		}
	}

	// Then — each of the four §13.2 systems appears in exactly one category
	allFour := []AgentSystemID{SafetySystemClaudeCode, SafetySystemSWEAgent, SafetySystemOpenHands, SafetySystemAider}
	for _, id := range allFour {
		count := seen[id]
		assert.Equal(t, 1, count,
			"system %s must appear in exactly one isolation-boundary category, got %d", id, count)
	}

	// And — total across all categories must equal 4
	totalSystems := 0
	for _, b := range allBoundaries {
		totalSystems += len(r.SystemsByIsolationBoundary(b))
	}
	assert.Equal(t, 4, totalSystems,
		"sum of all isolation-boundary groups must equal 4")
}

// Scenario 8: ML classifier uniquely distinguishes Claude Code from the field
// Given §5.3 documents yoloClassifier.ts as Claude Code's auto-mode safety mechanism
// When filtering by ML classifier presence and comparing with other safety axes
// Then only Claude Code uses an ML classifier
// And Claude Code is also the only system with layered policy enforcement
// And this combination (ML + layered) is unique in the §13.2 landscape
func TestBDD_Scenario8_MLClassifierUniquelyDistinguishesClaudeCode(t *testing.T) {
	// Given
	r := NewSystemSafetyComparisonRegistry()

	// When
	mlSystems := r.SystemsWithMLClassifier()
	layeredSystems := r.SystemsByIsolationBoundary(IsolationLayeredPolicyEnforcement)

	// Then
	require.Len(t, mlSystems, 1, "exactly one §13.2 system uses an ML classifier")
	assert.Equal(t, SafetySystemClaudeCode, mlSystems[0].SystemID,
		"the ML-classifier system must be Claude Code per §5.3")
	assert.True(t, mlSystems[0].UsesMLClassifier)

	// And
	require.Len(t, layeredSystems, 1, "exactly one §13.2 system uses layered policy enforcement")
	assert.Equal(t, SafetySystemClaudeCode, layeredSystems[0].SystemID)

	// And — the two filters agree: they both uniquely identify Claude Code
	assert.Equal(t, mlSystems[0].SystemID, layeredSystems[0].SystemID,
		"ML classifier and layered policy must be co-located in the same system")

	// And — all other systems lack both properties
	for _, id := range []AgentSystemID{SafetySystemSWEAgent, SafetySystemOpenHands, SafetySystemAider} {
		p, _ := r.FindByID(id)
		assert.False(t, p.UsesMLClassifier,
			"system %s must not use an ML classifier", id)
		assert.NotEqual(t, IsolationLayeredPolicyEnforcement, p.IsolationBoundary,
			"system %s must not use layered policy enforcement", id)
	}
}
