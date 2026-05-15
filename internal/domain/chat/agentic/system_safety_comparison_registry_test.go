package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// system_safety_comparison_registry_test.go — unit tests for FEAT-047
//
// Coverage targets:
//   - All public methods on SystemSafetyComparisonRegistry
//   - All four system profiles (FindByID)
//   - All cross-cutting filter methods
//   - CompareApprovalModels / CompareIsolationBoundaries / CompareRecoveryMechanisms
//   - SafestSystem / MostAutonomousSystem
//   - StructuralInvariants (4 invariants)
//   - IsValidSystemID for valid and invalid IDs
//   - SystemsByIsolationBoundary / SystemsByApprovalModel / SystemsByRecoveryMechanism

func newSafetyRegistry() *SystemSafetyComparisonRegistry {
	return NewSystemSafetyComparisonRegistry()
}

// --- Constructor and Count ---

func TestSystemSafetyRegistry_NewReturnsNonNil(t *testing.T) {
	r := newSafetyRegistry()
	require.NotNil(t, r)
}

func TestSystemSafetyRegistry_CountIsFour(t *testing.T) {
	r := newSafetyRegistry()
	assert.Equal(t, 4, r.Count())
}

// --- AllSystems canonical order ---

func TestSystemSafetyRegistry_AllSystemsReturnsExactlyFour(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.AllSystems()
	assert.Len(t, systems, 4)
}

func TestSystemSafetyRegistry_AllSystemsCanonicalOrderClaudeCodeFirst(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.AllSystems()
	assert.Equal(t, SafetySystemClaudeCode, systems[0].SystemID)
}

func TestSystemSafetyRegistry_AllSystemsCanonicalOrderSWEAgentSecond(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.AllSystems()
	assert.Equal(t, SafetySystemSWEAgent, systems[1].SystemID)
}

func TestSystemSafetyRegistry_AllSystemsCanonicalOrderOpenHandsThird(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.AllSystems()
	assert.Equal(t, SafetySystemOpenHands, systems[2].SystemID)
}

func TestSystemSafetyRegistry_AllSystemsCanonicalOrderAiderFourth(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.AllSystems()
	assert.Equal(t, SafetySystemAider, systems[3].SystemID)
}

// --- FindByID ---

func TestSystemSafetyRegistry_FindByIDClaudeCode(t *testing.T) {
	r := newSafetyRegistry()
	p, ok := r.FindByID(SafetySystemClaudeCode)
	require.True(t, ok)
	assert.Equal(t, SafetySystemClaudeCode, p.SystemID)
	assert.Equal(t, "Claude Code", p.SystemName)
	assert.Equal(t, ApprovalPerActionDenyFirst, p.ApprovalModel)
	assert.Equal(t, IsolationLayeredPolicyEnforcement, p.IsolationBoundary)
	assert.Equal(t, RecoverySessionScopedPermissionReset, p.RecoveryMechanism)
	assert.True(t, p.IsHumanInLoop)
	assert.True(t, p.DefaultsToSafe)
	assert.False(t, p.SupportsGitRollback)
	assert.True(t, p.UsesMLClassifier)
	assert.False(t, p.UsesContainerIsolation)
}

func TestSystemSafetyRegistry_FindByIDSWEAgent(t *testing.T) {
	r := newSafetyRegistry()
	p, ok := r.FindByID(SafetySystemSWEAgent)
	require.True(t, ok)
	assert.Equal(t, "SWE-Agent", p.SystemName)
	assert.Equal(t, ApprovalContainerBoundary, p.ApprovalModel)
	assert.Equal(t, IsolationDockerContainer, p.IsolationBoundary)
	assert.Equal(t, RecoveryContainerDispose, p.RecoveryMechanism)
	assert.False(t, p.IsHumanInLoop)
	assert.False(t, p.DefaultsToSafe)
	assert.True(t, p.UsesContainerIsolation)
}

func TestSystemSafetyRegistry_FindByIDOpenHands(t *testing.T) {
	r := newSafetyRegistry()
	p, ok := r.FindByID(SafetySystemOpenHands)
	require.True(t, ok)
	assert.Equal(t, "OpenHands", p.SystemName)
	assert.Equal(t, IsolationDockerContainer, p.IsolationBoundary)
	assert.True(t, p.UsesContainerIsolation)
	assert.False(t, p.UsesMLClassifier)
}

func TestSystemSafetyRegistry_FindByIDAider(t *testing.T) {
	r := newSafetyRegistry()
	p, ok := r.FindByID(SafetySystemAider)
	require.True(t, ok)
	assert.Equal(t, "Aider", p.SystemName)
	assert.Equal(t, ApprovalVersionControlAsNet, p.ApprovalModel)
	assert.Equal(t, IsolationNone, p.IsolationBoundary)
	assert.Equal(t, RecoveryGitRollback, p.RecoveryMechanism)
	assert.True(t, p.SupportsGitRollback)
	assert.False(t, p.IsHumanInLoop)
}

func TestSystemSafetyRegistry_FindByIDUnknownReturnsFalse(t *testing.T) {
	r := newSafetyRegistry()
	_, ok := r.FindByID("unknown_system")
	assert.False(t, ok)
}

// --- IsValidSystemID ---

func TestSystemSafetyRegistry_IsValidSystemIDTrueForAllFour(t *testing.T) {
	r := newSafetyRegistry()
	for _, id := range []AgentSystemID{SafetySystemClaudeCode, SafetySystemSWEAgent, SafetySystemOpenHands, SafetySystemAider} {
		assert.True(t, r.IsValidSystemID(id), "expected %s to be valid", id)
	}
}

func TestSystemSafetyRegistry_IsValidSystemIDFalseForUnknown(t *testing.T) {
	r := newSafetyRegistry()
	assert.False(t, r.IsValidSystemID("codex_cli"))
	assert.False(t, r.IsValidSystemID(""))
	assert.False(t, r.IsValidSystemID("devin"))
}

// --- SystemsWithHumanInLoop ---

func TestSystemSafetyRegistry_SystemsWithHumanInLoopOnlyClaudeCode(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsWithHumanInLoop()
	require.Len(t, systems, 1)
	assert.Equal(t, SafetySystemClaudeCode, systems[0].SystemID)
}

// --- SystemsDefaultingSafe ---

func TestSystemSafetyRegistry_SystemsDefaultingSafeOnlyClaudeCode(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsDefaultingSafe()
	require.Len(t, systems, 1)
	assert.Equal(t, SafetySystemClaudeCode, systems[0].SystemID)
}

// --- SystemsWithGitRollback ---

func TestSystemSafetyRegistry_SystemsWithGitRollbackOnlyAider(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsWithGitRollback()
	require.Len(t, systems, 1)
	assert.Equal(t, SafetySystemAider, systems[0].SystemID)
	assert.Equal(t, RecoveryGitRollback, systems[0].RecoveryMechanism)
}

// --- SystemsWithContainerIsolation ---

func TestSystemSafetyRegistry_SystemsWithContainerIsolationSWEAndOpenHands(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsWithContainerIsolation()
	require.Len(t, systems, 2)
	ids := []AgentSystemID{systems[0].SystemID, systems[1].SystemID}
	assert.Contains(t, ids, SafetySystemSWEAgent)
	assert.Contains(t, ids, SafetySystemOpenHands)
}

// --- SystemsWithMLClassifier ---

func TestSystemSafetyRegistry_SystemsWithMLClassifierOnlyClaudeCode(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsWithMLClassifier()
	require.Len(t, systems, 1)
	assert.Equal(t, SafetySystemClaudeCode, systems[0].SystemID)
}

// --- SafestSystem ---

func TestSystemSafetyRegistry_SafestSystemIsClaudeCode(t *testing.T) {
	r := newSafetyRegistry()
	p := r.SafestSystem()
	assert.Equal(t, SafetySystemClaudeCode, p.SystemID)
	assert.True(t, p.DefaultsToSafe)
	assert.True(t, p.IsHumanInLoop)
	assert.True(t, p.UsesMLClassifier)
}

// --- MostAutonomousSystem ---

func TestSystemSafetyRegistry_MostAutonomousSystemIsSWEAgent(t *testing.T) {
	r := newSafetyRegistry()
	p := r.MostAutonomousSystem()
	assert.Equal(t, SafetySystemSWEAgent, p.SystemID)
	assert.False(t, p.IsHumanInLoop)
}

// --- CompareApprovalModels ---

func TestSystemSafetyRegistry_CompareApprovalModelsReturnsFourEntries(t *testing.T) {
	r := newSafetyRegistry()
	entries := r.CompareApprovalModels()
	assert.Len(t, entries, 4)
}

func TestSystemSafetyRegistry_CompareApprovalModelsContainsClaudeCodeDenyFirst(t *testing.T) {
	r := newSafetyRegistry()
	entries := r.CompareApprovalModels()
	assert.Contains(t, entries[0], "Claude Code")
	assert.Contains(t, entries[0], string(ApprovalPerActionDenyFirst))
}

// --- CompareIsolationBoundaries ---

func TestSystemSafetyRegistry_CompareIsolationBoundariesReturnsFourEntries(t *testing.T) {
	r := newSafetyRegistry()
	entries := r.CompareIsolationBoundaries()
	assert.Len(t, entries, 4)
}

func TestSystemSafetyRegistry_CompareIsolationBoundariesAiderHasNone(t *testing.T) {
	r := newSafetyRegistry()
	entries := r.CompareIsolationBoundaries()
	// Aider is 4th in canonical order
	assert.Contains(t, entries[3], "Aider")
	assert.Contains(t, entries[3], string(IsolationNone))
}

// --- CompareRecoveryMechanisms ---

func TestSystemSafetyRegistry_CompareRecoveryMechanismsReturnsFourEntries(t *testing.T) {
	r := newSafetyRegistry()
	entries := r.CompareRecoveryMechanisms()
	assert.Len(t, entries, 4)
}

func TestSystemSafetyRegistry_CompareRecoveryMechanismsAiderGitRollback(t *testing.T) {
	r := newSafetyRegistry()
	entries := r.CompareRecoveryMechanisms()
	assert.Contains(t, entries[3], "Aider")
	assert.Contains(t, entries[3], string(RecoveryGitRollback))
}

// --- SystemsByIsolationBoundary ---

func TestSystemSafetyRegistry_SystemsByIsolationBoundaryDockerReturnsTwo(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsByIsolationBoundary(IsolationDockerContainer)
	assert.Len(t, systems, 2)
}

func TestSystemSafetyRegistry_SystemsByIsolationBoundaryLayeredReturnsClaudeCode(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsByIsolationBoundary(IsolationLayeredPolicyEnforcement)
	require.Len(t, systems, 1)
	assert.Equal(t, SafetySystemClaudeCode, systems[0].SystemID)
}

func TestSystemSafetyRegistry_SystemsByIsolationBoundaryUnknownReturnsEmpty(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsByIsolationBoundary("nonexistent")
	assert.Empty(t, systems)
}

// --- SystemsByApprovalModel ---

func TestSystemSafetyRegistry_SystemsByApprovalModelContainerBoundaryReturnsTwo(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsByApprovalModel(ApprovalContainerBoundary)
	assert.Len(t, systems, 2)
}

func TestSystemSafetyRegistry_SystemsByApprovalModelVersionControlReturnsAider(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsByApprovalModel(ApprovalVersionControlAsNet)
	require.Len(t, systems, 1)
	assert.Equal(t, SafetySystemAider, systems[0].SystemID)
}

// --- SystemsByRecoveryMechanism ---

func TestSystemSafetyRegistry_SystemsByRecoveryMechanismContainerDisposeReturnsTwo(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsByRecoveryMechanism(RecoveryContainerDispose)
	assert.Len(t, systems, 2)
}

func TestSystemSafetyRegistry_SystemsByRecoveryMechanismGitRollbackReturnsAider(t *testing.T) {
	r := newSafetyRegistry()
	systems := r.SystemsByRecoveryMechanism(RecoveryGitRollback)
	require.Len(t, systems, 1)
	assert.Equal(t, SafetySystemAider, systems[0].SystemID)
}

// --- PDFEvidence ---

func TestSystemSafetyRegistry_PDFEvidenceNonEmptyForAllSystems(t *testing.T) {
	r := newSafetyRegistry()
	for _, p := range r.AllSystems() {
		assert.NotEmpty(t, p.PDFEvidence, "expected PDFEvidence for %s", p.SystemID)
	}
}

func TestSystemSafetyRegistry_ClaudeCodePDFEvidenceMentionsDenyFirst(t *testing.T) {
	r := newSafetyRegistry()
	p, _ := r.FindByID(SafetySystemClaudeCode)
	assert.True(t, strings.Contains(p.PDFEvidence, "deny-first"))
}

func TestSystemSafetyRegistry_AiderPDFEvidenceMentionsGit(t *testing.T) {
	r := newSafetyRegistry()
	p, _ := r.FindByID(SafetySystemAider)
	assert.True(t, strings.Contains(p.PDFEvidence, "Git"))
}

// --- PDFSection ---

func TestSystemSafetyRegistry_AllSystemsHavePDFSection132(t *testing.T) {
	r := newSafetyRegistry()
	for _, p := range r.AllSystems() {
		assert.Equal(t, "13.2", p.PDFSection, "system %s should have PDFSection=13.2", p.SystemID)
	}
}

// --- StructuralInvariants ---

func TestSystemSafetyRegistry_StructuralInvariantsReturnsFourResults(t *testing.T) {
	r := newSafetyRegistry()
	results := r.StructuralInvariants()
	assert.Len(t, results, 4)
}

func TestSystemSafetyRegistry_InvariantExactlyFourSystemsHolds(t *testing.T) {
	r := newSafetyRegistry()
	results := r.StructuralInvariants()
	var found *SafetyInvariantResult
	for i := range results {
		if results[i].Name == "ExactlyFourSystems" {
			found = &results[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Holds, found.Message)
}

func TestSystemSafetyRegistry_InvariantClaudeCodeIsHumanInLoopHolds(t *testing.T) {
	r := newSafetyRegistry()
	results := r.StructuralInvariants()
	var found *SafetyInvariantResult
	for i := range results {
		if results[i].Name == "ClaudeCodeIsHumanInLoop" {
			found = &results[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Holds, found.Message)
}

func TestSystemSafetyRegistry_InvariantGitRollbackSystemsAreSafeHolds(t *testing.T) {
	r := newSafetyRegistry()
	results := r.StructuralInvariants()
	var found *SafetyInvariantResult
	for i := range results {
		if results[i].Name == "GitRollbackSystemsAreSafe" {
			found = &results[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Holds, found.Message)
}

func TestSystemSafetyRegistry_InvariantContainerSystemsNotDenyFirstHolds(t *testing.T) {
	r := newSafetyRegistry()
	results := r.StructuralInvariants()
	var found *SafetyInvariantResult
	for i := range results {
		if results[i].Name == "ContainerSystemsNotDenyFirst" {
			found = &results[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Holds, found.Message)
}

func TestSystemSafetyRegistry_AllInvariantsHold(t *testing.T) {
	r := newSafetyRegistry()
	for _, inv := range r.StructuralInvariants() {
		assert.True(t, inv.Holds, "invariant %s failed: %s", inv.Name, inv.Message)
	}
}

// --- Cross-system consistency ---

func TestSystemSafetyRegistry_SWEAgentAndOpenHandsShareDockerModel(t *testing.T) {
	r := newSafetyRegistry()
	swe, _ := r.FindByID(SafetySystemSWEAgent)
	oh, _ := r.FindByID(SafetySystemOpenHands)
	assert.Equal(t, swe.ApprovalModel, oh.ApprovalModel)
	assert.Equal(t, swe.IsolationBoundary, oh.IsolationBoundary)
	assert.Equal(t, swe.RecoveryMechanism, oh.RecoveryMechanism)
}

func TestSystemSafetyRegistry_ClaudeCodeUniqueInAllFilterDimensions(t *testing.T) {
	r := newSafetyRegistry()
	// Claude Code should be the only system in layered-policy, per-action, session-reset
	byIsolation := r.SystemsByIsolationBoundary(IsolationLayeredPolicyEnforcement)
	byApproval := r.SystemsByApprovalModel(ApprovalPerActionDenyFirst)
	byRecovery := r.SystemsByRecoveryMechanism(RecoverySessionScopedPermissionReset)
	assert.Len(t, byIsolation, 1)
	assert.Len(t, byApproval, 1)
	assert.Len(t, byRecovery, 1)
	assert.Equal(t, SafetySystemClaudeCode, byIsolation[0].SystemID)
	assert.Equal(t, SafetySystemClaudeCode, byApproval[0].SystemID)
	assert.Equal(t, SafetySystemClaudeCode, byRecovery[0].SystemID)
}

func TestSystemSafetyRegistry_AiderUniqueInVersionControlDimension(t *testing.T) {
	r := newSafetyRegistry()
	byApproval := r.SystemsByApprovalModel(ApprovalVersionControlAsNet)
	byIsolation := r.SystemsByIsolationBoundary(IsolationNone)
	assert.Len(t, byApproval, 1)
	assert.Len(t, byIsolation, 1)
	assert.Equal(t, SafetySystemAider, byApproval[0].SystemID)
	assert.Equal(t, SafetySystemAider, byIsolation[0].SystemID)
}

// --- PermissionGranularity ---

func TestSystemSafetyRegistry_ClaudeCodeHasPerActionGranularity(t *testing.T) {
	r := newSafetyRegistry()
	p, _ := r.FindByID(SafetySystemClaudeCode)
	assert.Equal(t, PermGranularityPerAction, p.PermissionGranularity)
}

func TestSystemSafetyRegistry_ContainerSystemsHaveEnvironmentLevelGranularity(t *testing.T) {
	r := newSafetyRegistry()
	for _, p := range r.SystemsWithContainerIsolation() {
		assert.Equal(t, PermGranularityEnvironmentLevel, p.PermissionGranularity,
			"container system %s should have environment-level granularity", p.SystemID)
	}
}

func TestSystemSafetyRegistry_AiderHasFileSystemGranularity(t *testing.T) {
	r := newSafetyRegistry()
	p, _ := r.FindByID(SafetySystemAider)
	assert.Equal(t, PermGranularityFileSystem, p.PermissionGranularity)
}
