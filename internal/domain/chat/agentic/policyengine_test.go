package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReq(tool string) PolicyEvaluationRequest {
	return PolicyEvaluationRequest{
		TenantID:  "tenant-x",
		AgentID:   "agent-y",
		ToolName:  tool,
		Timestamp: time.Now(),
	}
}

func TestPolicy_OutcomeEnumIsBounded(t *testing.T) {
	assert.True(t, IsTerminalPolicyOutcome(PolicyAllow))
	assert.True(t, IsTerminalPolicyOutcome(PolicyDeny))
	assert.True(t, IsTerminalPolicyOutcome(PolicyRequireApproval))
	assert.True(t, IsTerminalPolicyOutcome(PolicyIndeterminate))
	assert.False(t, IsTerminalPolicyOutcome(PolicyOutcome("unknown")))
	assert.False(t, IsTerminalPolicyOutcome(""))
}

func TestPolicy_NoOpAllowsEverything(t *testing.T) {
	eng := NewNoOpPolicyEngine()
	res, err := eng.Evaluate(context.Background(), newReq("any-tool"))
	require.NoError(t, err)
	assert.Equal(t, PolicyAllow, res.Outcome)
	assert.Equal(t, "noop", res.EngineName)
	assert.False(t, res.EvaluatedAt.IsZero())
}

func TestPolicy_RejectsRequestWithoutTenantID(t *testing.T) {
	eng := NewNoOpPolicyEngine()
	_, err := eng.Evaluate(context.Background(), PolicyEvaluationRequest{ToolName: "x"})
	assert.True(t, errors.Is(err, ErrNilPolicyRequest))
}

func TestPolicy_ContextCancelledReturnsError(t *testing.T) {
	eng := NewNoOpPolicyEngine()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := eng.Evaluate(ctx, newReq("x"))
	assert.Error(t, err)
}

// --- LimitsBackedPolicyEngine ---

type stubLimitsProvider struct {
	limits *PolicyLimits
}

func (s *stubLimitsProvider) CurrentLimits() *PolicyLimits { return s.limits }

func TestPolicy_LimitsBacked_NoLimitsFetchedReturnsIndeterminate(t *testing.T) {
	eng := NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: nil})
	res, err := eng.Evaluate(context.Background(), newReq("x"))
	require.NoError(t, err)
	assert.Equal(t, PolicyIndeterminate, res.Outcome,
		"no fetched limits → indeterminate, NOT silent allow")
	assert.NotEmpty(t, res.Reason)
}

func TestPolicy_LimitsBacked_NotInRestrictionsAllowed(t *testing.T) {
	limits := &PolicyLimits{
		Restrictions: map[string]PolicyRestriction{
			"banned-tool": {Allowed: false},
		},
	}
	eng := NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: limits})
	res, err := eng.Evaluate(context.Background(), newReq("safe-tool"))
	require.NoError(t, err)
	assert.Equal(t, PolicyAllow, res.Outcome,
		"absence-as-allowed semantics — only listed-deny rows block")
}

func TestPolicy_LimitsBacked_RestrictedToolDenied(t *testing.T) {
	limits := &PolicyLimits{
		Restrictions: map[string]PolicyRestriction{
			"banned-tool": {Allowed: false},
		},
	}
	eng := NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: limits})
	res, err := eng.Evaluate(context.Background(), newReq("banned-tool"))
	require.NoError(t, err)
	assert.Equal(t, PolicyDeny, res.Outcome)
	assert.Contains(t, res.Reason, "banned-tool",
		"deny reason must cite the tool name for audit clarity")
}

// --- StaticDenyPolicyEngine ---

func TestPolicy_StaticDeny_MapMatchesProduceDeny(t *testing.T) {
	eng := NewStaticDenyPolicyEngine("static", map[string]string{
		"shell": "shell access disabled enterprise-wide",
	})
	res, _ := eng.Evaluate(context.Background(), newReq("shell"))
	assert.Equal(t, PolicyDeny, res.Outcome)
	assert.Equal(t, "shell access disabled enterprise-wide", res.Reason)
}

func TestPolicy_StaticDeny_NonMatchAllow(t *testing.T) {
	eng := NewStaticDenyPolicyEngine("static", map[string]string{"shell": "x"})
	res, _ := eng.Evaluate(context.Background(), newReq("read-file"))
	assert.Equal(t, PolicyAllow, res.Outcome)
}

func TestPolicy_StaticDeny_AddDenyIsConcurrentSafe(t *testing.T) {
	eng := NewStaticDenyPolicyEngine("static", map[string]string{})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			eng.AddDeny("tool-"+string(rune('a'+(i%26))), "x")
			_, _ = eng.Evaluate(context.Background(), newReq("tool-a"))
		}(i)
	}
	wg.Wait() // -race must be clean
}

// --- ChainedPolicyEngine ---

func TestPolicy_Chain_FirstDenyWins(t *testing.T) {
	allow := NewNoOpPolicyEngine()
	deny := NewStaticDenyPolicyEngine("guard", map[string]string{"shell": "blocked"})
	chain := NewChainedPolicyEngine("ent-then-tenant", deny, allow)

	res, _ := chain.Evaluate(context.Background(), newReq("shell"))
	assert.Equal(t, PolicyDeny, res.Outcome)
	assert.Contains(t, res.EngineName, "guard")
}

func TestPolicy_Chain_AllAllowReturnsAllow(t *testing.T) {
	chain := NewChainedPolicyEngine("c", NewNoOpPolicyEngine(), NewNoOpPolicyEngine())
	res, _ := chain.Evaluate(context.Background(), newReq("anything"))
	assert.Equal(t, PolicyAllow, res.Outcome)
	assert.Equal(t, "c", res.EngineName)
}

func TestPolicy_Chain_AllIndeterminateReturnsIndeterminate(t *testing.T) {
	chain := NewChainedPolicyEngine("c",
		NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: nil}),
		NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: nil}),
	)
	res, _ := chain.Evaluate(context.Background(), newReq("x"))
	assert.Equal(t, PolicyIndeterminate, res.Outcome)
}

func TestPolicy_Chain_SkipsIndeterminateAndReturnsAllow(t *testing.T) {
	chain := NewChainedPolicyEngine("c",
		NewLimitsBackedPolicyEngine(&stubLimitsProvider{limits: nil}), // indeterminate
		NewNoOpPolicyEngine(), // allow
	)
	res, _ := chain.Evaluate(context.Background(), newReq("x"))
	assert.Equal(t, PolicyAllow, res.Outcome,
		"indeterminate must NOT block — next engine gets a chance")
}

func TestPolicy_Chain_ApprovalRequiredAfterAllAllowsTakesPrecedence(t *testing.T) {
	approval := approvalEngine{name: "approval"}
	chain := NewChainedPolicyEngine("c", NewNoOpPolicyEngine(), approval)
	res, _ := chain.Evaluate(context.Background(), newReq("x"))
	assert.Equal(t, PolicyRequireApproval, res.Outcome,
		"require_approval beats allow in the chain (stricter wins)")
}

func TestPolicy_Chain_DenyBeatsApproval(t *testing.T) {
	deny := NewStaticDenyPolicyEngine("guard", map[string]string{"x": "no"})
	approval := approvalEngine{name: "approval"}
	chain := NewChainedPolicyEngine("c", approval, deny)
	res, _ := chain.Evaluate(context.Background(), newReq("x"))
	assert.Equal(t, PolicyDeny, res.Outcome,
		"deny is the strictest — beats require_approval")
}

func TestPolicy_Chain_PropagatesUnderlyingError(t *testing.T) {
	failing := failingEngine{}
	chain := NewChainedPolicyEngine("c", failing, NewNoOpPolicyEngine())
	_, err := chain.Evaluate(context.Background(), newReq("x"))
	assert.Error(t, err, "underlying engine error must NOT be silently swallowed")
}

func TestPolicy_Chain_UnionsObligations(t *testing.T) {
	chain := NewChainedPolicyEngine("c",
		obligationEngine{name: "a", obs: []string{"audit_log"}},
		obligationEngine{name: "b", obs: []string{"evidence_header"}},
	)
	res, _ := chain.Evaluate(context.Background(), newReq("x"))
	assert.Equal(t, PolicyAllow, res.Outcome)
	assert.ElementsMatch(t,
		[]string{"audit_log", "evidence_header"}, res.Obligations,
		"obligations from all allow-engines must be unioned")
}

// --- test helpers ---

type approvalEngine struct{ name string }

func (a approvalEngine) Name() string { return a.name }
func (a approvalEngine) Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error) {
	return PolicyEvaluationResult{
		Outcome:     PolicyRequireApproval,
		Reason:      "human approval required by policy",
		EngineName:  a.name,
		EvaluatedAt: time.Now(),
	}, nil
}

type failingEngine struct{}

func (failingEngine) Name() string { return "failing" }
func (failingEngine) Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error) {
	return PolicyEvaluationResult{}, errors.New("backend unreachable")
}

type obligationEngine struct {
	name string
	obs  []string
}

func (o obligationEngine) Name() string { return o.name }
func (o obligationEngine) Evaluate(ctx context.Context, req PolicyEvaluationRequest) (PolicyEvaluationResult, error) {
	return PolicyEvaluationResult{
		Outcome:     PolicyAllow,
		Obligations: o.obs,
		EngineName:  o.name,
		EvaluatedAt: time.Now(),
	}, nil
}
