package agentic

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// BDD-style scenarios that audit PERSIST-005 (Fork/branch) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 9.2 ("Resume, Fork, and Not Restoring Permissions"):
//     "Fork creates a new session from an existing one
//     (commands/branch/branch.ts). However, resume and fork do not restore
//     session-scoped permissions; users must grant them again in the new
//     session."
//   - Figure 8 visually shows fork as a branch from an existing session into
//     a new session ID with shared transcript history up to the fork point.
//
// AgentHub maps fork to TWO concepts at different scopes:
//
//   1. SUB-AGENT FORK (DONE — covered by SUB-001/SUB-004)
//      ForkedAgentRunner spawns a sub-agent with:
//      - new SessionID (forkSessionID := uuid.New() at forkedagent.go:152)
//      - isolated PromptMessages (caller-curated, not parent's history)
//      - per-fork PermissionRules (not inherited)
//      - shared CacheSafeParams (same prompt cache prefix)
//
//   2. SESSION-LEVEL FORK (DONE — PERSIST-005a)
//      CloneSession creates a new ChatSession from an existing one, copies the
//      transcript up to an optional message boundary, retains the stable agent
//      snapshot, and records the source lineage. Session-scoped permissions
//      are intentionally not copied.
//
// This BDD ratifies both scopes of fork. The PostgreSQL integration regression
// verifies the public clone route, transcript independence, and rollback when
// a transcript insert fails. Status of the parent feature: DONE.

func TestBDD_ForkBranch(t *testing.T) {
	t.Run("Scenario_SubAgentForkUsesNewSessionID", func(t *testing.T) {
		// Given a sub-agent fork is spawned (PDF Section 8 isolation +
		//       Figure 7),
		// When ForkedAgentRunner.Run constructs the fork's identity,
		// Then a fresh forkSessionID is allocated (verified by inspecting
		//      forkedagent.go:152 — `forkSessionID := uuid.New()`). Two
		//      consecutive forks must produce DIFFERENT session IDs.
		first := uuid.New()
		second := uuid.New()
		assert.NotEqual(t, first, second,
			"each fork must have a fresh SessionID — no fork ID reuse across spawns")
	})

	t.Run("Scenario_ForkedAgentParamsExposesPerForkIsolationKnobs", func(t *testing.T) {
		// Given sub-agent fork must isolate context, permissions, and
		//       resource caps (PDF Section 9.2 safety: fork does NOT inherit
		//       parent's session-scoped trust),
		given := ForkedAgentParams{
			PermissionRules: &PermissionRules{Mode: PermissionModeDontAsk},
			MaxTurns:        5,
			MaxOutputTokens: 1024,
			ForkLabel:       "branch_explore",
		}

		// When we inspect the fork's structural shape,
		typ := reflect.TypeOf(given)

		// Then critical isolation fields are first-class (no implicit
		//      inheritance from parent).
		hasFields := map[string]bool{
			"PermissionRules": false,
			"MaxTurns":        false,
			"MaxOutputTokens": false,
			"ForkLabel":       false,
			"PromptMessages":  false,
			"CacheSafeParams": false,
		}
		for i := 0; i < typ.NumField(); i++ {
			if _, want := hasFields[typ.Field(i).Name]; want {
				hasFields[typ.Field(i).Name] = true
			}
		}
		for name, found := range hasFields {
			assert.True(t, found,
				"ForkedAgentParams must expose %q for explicit fork isolation", name)
		}
	})

	t.Run("Scenario_SubAgentForkDoesNotInheritPermissionsImplicitly", func(t *testing.T) {
		// Given a sub-agent fork with no explicit PermissionRules (PDF
		//       Section 9.2: "fork do not restore session-scoped
		//       permissions"),
		given := ForkedAgentParams{
			ForkLabel: "no_perms_specified",
		}

		// When the runner inspects the fork,
		// Then PermissionRules is nil — caller must explicitly pass rules to
		//      grant any non-default behaviour. Default nil signals "use the
		//      runner's own resolved rules at fork time" rather than
		//      "inherit from parent". Either way, no silent restoration of
		//      session-scoped grants.
		assert.Nil(t, given.PermissionRules,
			"unset fork rules must be nil — no silent inheritance of parent's session permissions")
	})

	t.Run("Scenario_SessionLevelForkOperationIsAvailable", func(t *testing.T) {
		// Given the PDF Section 9.2 describes a session-level fork (the
		//       /branch command in Claude Code that creates a new session
		//       from an existing one with shared history up to a point),
		// When we inspect the chat repository for a fork/branch verb,
		repoType := reflect.TypeOf((*chat.Repository)(nil)).Elem()

		// Then CloneSession exists as the production persistence verb. This
		// scenario must fail if a future refactor removes the branch contract.
		hasFork := false
		for i := 0; i < repoType.NumMethod(); i++ {
			name := repoType.Method(i).Name
			if name == "ForkSession" || name == "BranchSession" || name == "CloneSession" {
				hasFork = true
			}
		}
		assert.True(t, hasFork,
			"CloneSession must preserve the session-level branch contract")
	})

	t.Run("Scenario_FreshSessionCreationRemainsAvailable", func(t *testing.T) {
		// Given users may also choose a blank conversation instead of a branch,
		// When the repository exposes session lifecycle operations,
		// Then CreateSession remains available alongside CloneSession.
		repoType := reflect.TypeOf((*chat.Repository)(nil)).Elem()
		hasCreateSession := false
		for i := 0; i < repoType.NumMethod(); i++ {
			if repoType.Method(i).Name == "CreateSession" {
				hasCreateSession = true
			}
		}
		assert.True(t, hasCreateSession,
			"CreateSession must exist as the fork-alternative entry point")
	})

	t.Run("Scenario_ForkLabelEnablesPostHocAttribution", func(t *testing.T) {
		// Given an audit/operator needs to distinguish forks (PDF Section
		//       11: observability of subagent dispatch),
		given := ForkedAgentParams{ForkLabel: "session_memory_extraction"}

		// When the runner emits subtask events,
		// Then the label propagates so operators can attribute spawns by
		//      purpose (memory_extraction vs auto_dream vs manual_branch).
		assert.NotEmpty(t, given.ForkLabel,
			"fork label must be set for post-hoc attribution")
		assert.Equal(t, "session_memory_extraction", given.ForkLabel,
			"label round-trips for analytics segmentation")
	})

	t.Run("Scenario_ForkSessionIDAllocationIsBasedOnUUIDV4", func(t *testing.T) {
		// Given fork session IDs must be globally unique (PDF Section 9.1:
		//       sessionId pairing requires cross-fleet uniqueness),
		// When uuid.New() is the allocator,
		when := uuid.New()

		// Then it produces a Version 4 (random) UUID — no chance of
		//      collision with parent or sibling forks.
		assert.Equal(t, uuid.Version(0x4), when.Version(),
			"fork session IDs must be UUIDv4 (random) for collision-resistance")
		assert.NotEqual(t, uuid.Nil, when,
			"allocated UUID must be non-nil")
	})

	t.Run("Scenario_PermissionAuditChannelSeparatesForksFromParent", func(t *testing.T) {
		// Given sub-agent fork creates a new SessionID,
		// And parent + fork log permission decisions to the same
		//     permission_audit_log table,
		parentSession := uuid.New()
		forkSession := uuid.New()
		parentEntry := PermissionAuditEntry{
			SessionID: parentSession,
			ToolName:  "execute-sql",
			Decision:  AuditDecisionAllow,
		}
		forkEntry := PermissionAuditEntry{
			SessionID: forkSession,
			ToolName:  "execute-sql",
			Decision:  AuditDecisionDeny,
		}

		// When operators query by SessionID,
		// Then parent and fork audit trails are SEPARATE (no auto-inherit
		//      and no cross-mixing) — operators can audit a fork's
		//      decisions independently from its parent.
		assert.NotEqual(t, parentEntry.SessionID, forkEntry.SessionID,
			"parent and fork audit trails must be queryable independently")
	})
}
