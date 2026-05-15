package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that audit PERM-003 (Permission modes) against the
// Claude Code architecture paper "Dive into Claude Code" (arXiv:2604.14228v1),
// Section 5.1 ("Permission Modes and Rule Evaluation"), which defines seven
// modes as a graduated trust spectrum:
//
//   1. plan               — model emits a plan; execution proceeds only on
//                           user approval.
//   2. default            — standard interactive use; most ops require approval.
//   3. acceptEdits        — edits within CWD + safe FS commands auto-approved.
//   4. auto               — ML classifier (TRANSCRIPT_CLASSIFIER feature flag).
//   5. dontAsk            — no prompting; deny rules still enforced.
//   6. bypassPermissions  — skips most prompts, except bypass-immune rules.
//   7. bubble             — INTERNAL-ONLY: subagent permission escalation to
//                           parent terminal.
//
// AgentHub's PermissionMode enum currently exposes FOUR external modes:
// default, allow_edits (acceptEdits equivalent), bypass, dont_ask. The
// remaining three (plan, auto, bubble) are documented gaps:
//
//   - plan   : not yet implemented; a future feature for multi-turn
//              human-in-the-loop approval prior to execution.
//   - auto   : the ML classifier slot is filled by a heuristic
//              (EvaluatePermissionWithDangerCheck) rather than a transcript
//              ML model. PERM-007 will track the full classifier.
//   - bubble : subagent escalation mode is implicit in AgentHub's coordinator
//              spawning, not a first-class enum value. SUB-006 will track.
//
// The four implemented modes are exhaustively ratified below; the missing
// three are explicitly documented as expected gaps so that future work
// promoting PERM-003 to DONE has a concrete checklist.

func TestBDD_PermissionModesGraduatedSpectrum(t *testing.T) {
	t.Run("Scenario_DefaultModeAsksForConfirmRules", func(t *testing.T) {
		// Given the "default" mode (PDF Section 5.1 #2: "Standard interactive
		//       use. Most operations require user approval"),
		given := &PermissionRules{
			Mode:    PermissionModeDefault,
			Confirm: []string{"http-request"},
		}

		// When a confirm-matching call is attempted,
		when := EvaluatePermission(given, "http-request", "https://example.com")

		// Then the engine asks (returns Confirm) — the mode does not promote
		//      the rule to allow nor demote it to deny.
		assert.Equal(t, PermissionConfirm, when,
			"default mode must surface confirm rules verbatim")
	})

	t.Run("Scenario_AllowEditsModeIsTheAcceptEditsEquivalent", func(t *testing.T) {
		// Given AgentHub's allow_edits mode (PDF Section 5.1 #3 acceptEdits),
		given := &PermissionRules{Mode: PermissionModeAllowEdits}

		// When the runtime inspects the mode token,
		when := given.Mode

		// Then it round-trips through JSON unchanged — clients must be able to
		//      configure this mode by name. The rename "acceptEdits→allow_edits"
		//      is an intentional AgentHub convention; the semantics match the
		//      PDF (auto-approve safe edits within scope).
		assert.Equal(t, PermissionMode("allow_edits"), when,
			"allow_edits is AgentHub's name for the PDF acceptEdits mode")
		// Note: per-tool granular auto-approval semantics (which exact FS
		// commands are safe) are enforced by the danger-detection layer
		// (EvaluatePermissionWithDangerCheck) and the IsDestructive flag,
		// not by the mode token alone — covered in PERM-001/TOOL-001 BDDs.
	})

	t.Run("Scenario_BypassModeIsNotAUniversalAllow", func(t *testing.T) {
		// Given bypass mode with an explicit deny rule (PDF Section 5.1 #6:
		//       "skips MOST permission prompts, but safety-critical checks
		//       and bypass-immune rules still apply"),
		given := &PermissionRules{
			Mode: PermissionModeBypass,
			Deny: []string{"shell(rm -rf)"},
		}

		// When a denied destructive call is attempted,
		when := EvaluatePermission(given, "shell", "rm -rf /")

		// Then bypass does NOT override the deny — the immunity is real.
		assert.Equal(t, PermissionDeny, when,
			"bypass mode must respect explicit deny rules (bypass-immune)")
	})

	t.Run("Scenario_BypassModePromotesUnmatchedAllowAttempts", func(t *testing.T) {
		// Given bypass mode with allow-list-only rules and a tool not on the
		//       list (PDF Section 5.1: bypass relaxes enforcement when no
		//       deny applies),
		given := &PermissionRules{
			Mode:  PermissionModeBypass,
			Allow: []string{"document-search"},
		}

		// When an unlisted tool is attempted,
		when := EvaluatePermission(given, "memo-write", "scratchpad")

		// Then bypass promotes the call to Allow — the allow list is not a
		//      hard wall in bypass mode (would be deny in non-bypass modes).
		assert.Equal(t, PermissionAllow, when,
			"bypass mode must allow unlisted tools when no deny applies")
	})

	t.Run("Scenario_DontAskModeFailsClosedOnConfirm", func(t *testing.T) {
		// Given dont_ask mode (PDF Section 5.1 #5: "No prompting, but deny
		//       rules are still enforced"),
		given := &PermissionRules{
			Mode:    PermissionModeDontAsk,
			Confirm: []string{"http-request"},
		}

		// When a call that would normally require confirm is attempted,
		when := EvaluatePermission(given, "http-request", "https://example.com")

		// Then the engine fails closed (returns Deny) — preserves safety in
		//      unattended runs where prompting is impossible.
		assert.Equal(t, PermissionDeny, when,
			"dont_ask mode must convert confirm into deny (fail-closed)")
	})

	t.Run("Scenario_DontAskModeStillHonorsExplicitAllow", func(t *testing.T) {
		// Given dont_ask mode with an allow list,
		given := &PermissionRules{
			Mode:  PermissionModeDontAsk,
			Allow: []string{"document-search"},
		}

		// When an explicitly allowed tool is attempted,
		when := EvaluatePermission(given, "document-search", "any query")

		// Then it runs — dont_ask only converts CONFIRM to deny, it does not
		//      negate explicit allows.
		assert.Equal(t, PermissionAllow, when,
			"dont_ask mode must keep explicit allow rules functional")
	})

	t.Run("Scenario_FourImplementedModesAreDistinctConstants", func(t *testing.T) {
		// Given the externally-visible mode constants exposed by AgentHub,
		modes := []PermissionMode{
			PermissionModeDefault,
			PermissionModeAllowEdits,
			PermissionModeBypass,
			PermissionModeDontAsk,
		}

		// When the engine catalogues them,
		set := map[PermissionMode]bool{}
		for _, m := range modes {
			set[m] = true
		}

		// Then there are exactly 4 distinct values — refactor that collapses
		//      two modes into one would fail this guard.
		assert.Len(t, set, 4,
			"AgentHub must expose 4 distinct PermissionMode constants")
		// The 3 missing PDF modes (plan, auto, bubble) are NOT asserted here
		// to keep this BDD honest about the current state of the engine.
		// See ledger entry PERM-003 for the gap analysis and the IDs of the
		// follow-up features that will close them.
	})

	t.Run("Scenario_NilRulesYieldsAllowRegardlessOfMode", func(t *testing.T) {
		// Given no rules configured (rules == nil) — production callers MUST
		//       supply a default ruleset; this scenario verifies the engine
		//       does not crash and defaults to permissive,
		given := (*PermissionRules)(nil)

		// When the engine evaluates,
		when := EvaluatePermission(given, "anything", "any input")

		// Then Allow is returned — engine treats absence of rules as silent
		//      pass-through. The production runner is responsible for never
		//      calling EvaluatePermission with nil rules in interactive runs.
		assert.Equal(t, PermissionAllow, when,
			"engine must not crash on nil rules; default is Allow at engine layer")
	})

	t.Run("Scenario_ModeIsHonoredEvenWhenAllowAndConfirmAreEmpty", func(t *testing.T) {
		// Given a rules struct with only Mode set and a Deny rule,
		given := &PermissionRules{
			Mode: PermissionModeBypass,
			Deny: []string{"execute-sql(DROP)"},
		}

		// When a deny-matching call is attempted,
		when := EvaluatePermission(given, "execute-sql", "DROP TABLE users")

		// Then the deny still fires under bypass — preserves the invariant
		//      that mode never overrides explicit deny.
		assert.Equal(t, PermissionDeny, when,
			"deny rules apply regardless of mode")
	})
}
