package agentic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GOV-002 — External policy integration BDD.
//
// PDF arXiv:2604.14228v1 Section 11 — enterprises need pluggable policy
// engines (OPA, Cedar, custom). The runner must NOT hardcode rules.
// Section 5.3 — every decision (allow/deny/approval) is observable via
// the audit layer regardless of the deciding backend.
//
// These scenarios validate the integration contract: pluggable, chained
// (first-deny-wins + obligations-unioned), context-cancellation-honoring,
// indeterminate-without-silent-allow, and audit-friendly.

func TestBDD_ExternalPolicyIntegration(t *testing.T) {

	t.Run("Scenario_DefaultDeploymentAllowsEverythingViaNoOp", func(t *testing.T) {
		// Given a fresh AgentHub install (no enterprise policy backend),
		// When the runner asks the engine for a decision,
		// Then NoOp returns Allow — agents work out of the box.
		eng := NewNoOpPolicyEngine()
		res, err := eng.Evaluate(context.Background(), newReq("anything"))
		require.NoError(t, err)
		assert.Equal(t, PolicyAllow, res.Outcome)
	})

	t.Run("Scenario_EnterpriseLimitsBlockSpecificTools", func(t *testing.T) {
		// Given an enterprise admin pushed PolicyLimits restricting
		//       "shell" tool,
		limits := &PolicyLimits{Restrictions: map[string]PolicyRestriction{
			"shell": {Allowed: false},
		}}
		eng := NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: limits})

		// When the agent tries to invoke shell,
		res, _ := eng.Evaluate(context.Background(), newReq("shell"))

		// Then policy denies with a citing reason — audit log is informative.
		assert.Equal(t, PolicyDeny, res.Outcome)
		assert.Contains(t, res.Reason, "shell")
	})

	t.Run("Scenario_AbsenceMeansAllowedNotMissingPolicy", func(t *testing.T) {
		// Given enterprise PolicyLimits ONLY lists denied items
		//       (absence-as-allowed semantics — already in policylimits.go),
		limits := &PolicyLimits{Restrictions: map[string]PolicyRestriction{}}
		eng := NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: limits})

		// When a tool not in the deny list is queried,
		// Then it is allowed — empty restrictions ≠ deny everything.
		res, _ := eng.Evaluate(context.Background(), newReq("read-doc"))
		assert.Equal(t, PolicyAllow, res.Outcome)
	})

	t.Run("Scenario_NoLimitsFetchedYieldsIndeterminateNotSilentAllow", func(t *testing.T) {
		// Given a deployment where PolicyLimits has not yet been
		//       fetched (network down, cold start),
		eng := NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: nil})

		// When the engine is queried,
		// Then result is INDETERMINATE — never a silent allow. Caller
		//      decides fail-open vs fail-closed via chain composition.
		res, _ := eng.Evaluate(context.Background(), newReq("x"))
		assert.Equal(t, PolicyIndeterminate, res.Outcome,
			"missing data must be EXPLICIT — silent allow is the bug class GOV-002 prevents")
		assert.NotEmpty(t, res.Reason, "indeterminate must explain why")
	})

	t.Run("Scenario_ChainedEnterpriseThenTenantRespectsFirstDeny", func(t *testing.T) {
		// Given an enterprise blocks shell + tenant adds extra rules,
		entLimits := &PolicyLimits{Restrictions: map[string]PolicyRestriction{
			"shell": {Allowed: false},
		}}
		enterprise := NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: entLimits})
		tenant := NewStaticDenyPolicyEngine("tenant", map[string]string{
			"send-email": "tenant disabled outbound email",
		})
		chain := NewChainedPolicyEngine("ent-then-tenant", enterprise, tenant)

		// When agent asks for shell (enterprise denies first),
		shellRes, _ := chain.Evaluate(context.Background(), newReq("shell"))
		assert.Equal(t, PolicyDeny, shellRes.Outcome)
		assert.Contains(t, shellRes.EngineName, "policy-limits",
			"audit must record WHO denied — first matching engine name")

		// When agent asks for send-email (only tenant denies),
		emailRes, _ := chain.Evaluate(context.Background(), newReq("send-email"))
		assert.Equal(t, PolicyDeny, emailRes.Outcome)
		assert.Contains(t, emailRes.EngineName, "tenant",
			"second engine in chain attributed correctly")
	})

	t.Run("Scenario_ApprovalRequiredBlocksUntilHumanResponds", func(t *testing.T) {
		// Given a regulated tool (e.g. PII export) requires an
		//       approval token,
		chain := NewChainedPolicyEngine("c",
			NewNoOpPolicyEngine(),
			approvalEngine{name: "compliance"},
		)

		// When the agent attempts the tool,
		res, _ := chain.Evaluate(context.Background(), newReq("export-pii"))

		// Then the chain returns RequireApproval — runner must NOT
		//      execute, must surface the approval requirement to UI/ops.
		assert.Equal(t, PolicyRequireApproval, res.Outcome)
		assert.NotEmpty(t, res.Reason)
	})

	t.Run("Scenario_DenyWinsOverApprovalInChainHierarchy", func(t *testing.T) {
		// Given a stricter enterprise rule should always trump softer
		//       approval requirements (PDF Section 11 — strictest wins),
		chain := NewChainedPolicyEngine("c",
			approvalEngine{name: "soft"},
			NewStaticDenyPolicyEngine("hard", map[string]string{"x": "blocked"}),
		)

		// When evaluation runs both,
		res, _ := chain.Evaluate(context.Background(), newReq("x"))

		// Then deny wins.
		assert.Equal(t, PolicyDeny, res.Outcome,
			"hierarchy: deny > require_approval > allow > indeterminate")
	})

	t.Run("Scenario_ObligationsAccumulateAcrossChainWhenAllAllow", func(t *testing.T) {
		// Given multiple engines each impose obligations on allow
		//       (e.g. audit-log + evidence-header),
		chain := NewChainedPolicyEngine("c",
			obligationEngine{name: "audit", obs: []string{"audit_log"}},
			obligationEngine{name: "compliance", obs: []string{"evidence_header"}},
		)

		// When evaluation runs,
		res, _ := chain.Evaluate(context.Background(), newReq("x"))

		// Then BOTH obligations reach the runner — no engine's
		//      obligation gets dropped.
		assert.Equal(t, PolicyAllow, res.Outcome)
		assert.ElementsMatch(t,
			[]string{"audit_log", "evidence_header"}, res.Obligations,
			"obligations are UNION not single-engine — runner must satisfy all")
	})

	t.Run("Scenario_OutcomeEnumIsBoundedForAuditLayer", func(t *testing.T) {
		// Given the audit/tracing layer binds to outcome strings,
		// When the bounded set is inspected,
		// Then exactly 4 valid outcomes exist.
		valid := []PolicyOutcome{PolicyAllow, PolicyDeny, PolicyRequireApproval, PolicyIndeterminate}
		for _, o := range valid {
			assert.True(t, IsTerminalPolicyOutcome(o),
				"outcome %q must be in bounded set", o)
		}
		assert.False(t, IsTerminalPolicyOutcome(PolicyOutcome("any-other")),
			"unknown outcomes rejected by guard")
	})

	t.Run("Scenario_DecisionEnvelopeIsJSONStableForAuditWire", func(t *testing.T) {
		// Given the policy decision flows through audit logs / SSE,
		res := PolicyEvaluationResult{
			Outcome:     PolicyDeny,
			Reason:      "blocked",
			Obligations: []string{"log_to_compliance"},
			EngineName:  "chain:enterprise",
			EvaluatedAt: time.Now(),
			LatencyMs:   3,
		}
		raw, err := json.Marshal(res)
		require.NoError(t, err)
		s := string(raw)
		// Field names are the wire contract.
		assert.Contains(t, s, `"Outcome"`)
		assert.Contains(t, s, `"Reason"`)
		assert.Contains(t, s, `"Obligations"`)
		assert.Contains(t, s, `"EngineName"`)
		assert.Contains(t, s, `"EvaluatedAt"`)
		assert.Contains(t, s, `"LatencyMs"`)
	})

	t.Run("Scenario_ContextCancellationStopsChainMidEvaluation", func(t *testing.T) {
		// Given a chain with a slow engine and a cancellation,
		// When ctx cancels before the next engine runs,
		// Then the chain stops and propagates the cancel error — no
		//      further engines are invoked.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		chain := NewChainedPolicyEngine("c", NewNoOpPolicyEngine(), NewNoOpPolicyEngine())
		_, err := chain.Evaluate(ctx, newReq("x"))
		assert.Error(t, err)
	})

	t.Run("Scenario_RequestRejectedWithoutTenantIDPreventsBypass", func(t *testing.T) {
		// Given multi-tenant isolation requires every decision to be
		//       tenant-scoped (CLAUDE.md security boundary),
		// When a caller forgets to set TenantID,
		// Then the engine REJECTS with ErrNilPolicyRequest — never
		//      silently evaluates as a global decision.
		eng := NewNoOpPolicyEngine()
		_, err := eng.Evaluate(context.Background(), PolicyEvaluationRequest{ToolName: "x"})
		assert.Error(t, err,
			"missing TenantID must error — silent eval would breach tenant isolation")
	})

	t.Run("Scenario_AuditTrailRecordsEngineOriginEvenInChain", func(t *testing.T) {
		// Given the audit log needs to know which engine actually
		//       made the call (for compliance attribution),
		chain := NewChainedPolicyEngine("ent-then-tenant",
			NewStaticDenyPolicyEngine("ent-rule", map[string]string{"shell": "no"}),
			NewNoOpPolicyEngine(),
		)
		res, _ := chain.Evaluate(context.Background(), newReq("shell"))

		// Then EngineName carries chain:engine attribution.
		assert.Contains(t, res.EngineName, "ent-then-tenant",
			"chain name in attribution")
		assert.Contains(t, res.EngineName, "ent-rule",
			"deciding-engine name in attribution")
	})
}
