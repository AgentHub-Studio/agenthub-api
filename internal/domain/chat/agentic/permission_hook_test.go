package agentic

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermHook_IsValidPhase(t *testing.T) {
	for _, p := range allPermissionHookPhases {
		assert.True(t, IsValidPermissionHookPhase(p))
	}
	assert.False(t, IsValidPermissionHookPhase(PermissionHookPhase("nope")))
}

func TestPermHook_IsValidOutcome(t *testing.T) {
	for _, o := range allPermissionHookOutcomes {
		assert.True(t, IsValidPermissionHookOutcome(o))
	}
	assert.False(t, IsValidPermissionHookOutcome(PermissionHookOutcome("nope")))
}

func TestPermHook_ValidateNameRequired(t *testing.T) {
	h := PermissionHook{Phase: PermissionHookBeforeEvaluate, Handle: noopHookHandle}
	assert.ErrorIs(t, h.Validate(), ErrPermissionHookNameRequired)
}

func TestPermHook_ValidateBadPhase(t *testing.T) {
	h := PermissionHook{Name: "x", Phase: PermissionHookPhase("nope"), Handle: noopHookHandle}
	assert.ErrorIs(t, h.Validate(), ErrPermissionHookBadPhase)
}

func TestPermHook_ValidateHandleRequired(t *testing.T) {
	h := PermissionHook{Name: "x", Phase: PermissionHookBeforeEvaluate}
	assert.ErrorIs(t, h.Validate(), ErrPermissionHookHandleRequired)
}

func TestPermHook_RegisterAndCount(t *testing.T) {
	c := NewPermissionHookChain(nil)
	require.NoError(t, c.Register(PermissionHook{
		Name: "a", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: noopHookHandle,
	}))
	assert.Equal(t, 1, c.HookCount())
}

func TestPermHook_RegisterDuplicateInSamePhaseFails(t *testing.T) {
	c := NewPermissionHookChain(nil)
	require.NoError(t, c.Register(PermissionHook{
		Name: "a", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: noopHookHandle,
	}))
	err := c.Register(PermissionHook{
		Name: "a", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: noopHookHandle,
	})
	assert.ErrorIs(t, err, ErrPermissionHookDuplicate)
}

func TestPermHook_RegisterSameNameDifferentPhasesOK(t *testing.T) {
	c := NewPermissionHookChain(nil)
	require.NoError(t, c.Register(PermissionHook{
		Name: "a", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: noopHookHandle,
	}))
	require.NoError(t, c.Register(PermissionHook{
		Name: "a", Phase: PermissionHookAfterEvaluate,
		Enabled: true, Handle: noopHookHandle,
	}))
	assert.Equal(t, 2, c.HookCount())
}

func TestPermHook_RemoveDeletesHook(t *testing.T) {
	c := NewPermissionHookChain(nil)
	require.NoError(t, c.Register(PermissionHook{
		Name: "a", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: noopHookHandle,
	}))
	require.NoError(t, c.Remove("a", PermissionHookBeforeEvaluate))
	assert.Equal(t, 0, c.HookCount())
}

func TestPermHook_RemoveUnknownFails(t *testing.T) {
	c := NewPermissionHookChain(nil)
	err := c.Remove("nope", PermissionHookBeforeEvaluate)
	assert.ErrorIs(t, err, ErrPermissionHookNotFound)
}

func TestPermHook_DisableSkipsHookInChain(t *testing.T) {
	c := NewPermissionHookChain(nil)
	require.NoError(t, c.Register(PermissionHook{
		Name: "deny-all", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: overrideDenyHandle,
	}))
	c.Disable("deny-all", PermissionHookBeforeEvaluate)
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, PermissionAllow, eval.FinalDecision)
	assert.False(t, eval.ShortCircuited)
}

func TestPermHook_EnableReactivatesHook(t *testing.T) {
	c := NewPermissionHookChain(nil)
	require.NoError(t, c.Register(PermissionHook{
		Name: "deny-all", Phase: PermissionHookBeforeEvaluate,
		Enabled: false, Handle: overrideDenyHandle,
	}))
	c.Enable("deny-all", PermissionHookBeforeEvaluate)
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, PermissionDeny, eval.FinalDecision)
	assert.True(t, eval.ShortCircuited)
}

func TestPermHook_NoHooksReturnsEngineDecision(t *testing.T) {
	c := NewPermissionHookChain(&PermissionRules{Deny: []string{"Bash"}})
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
	assert.Equal(t, PermissionDeny, eval.EngineDecision)
	assert.Equal(t, PermissionDeny, eval.FinalDecision)
	assert.False(t, eval.ShortCircuited)
	assert.Empty(t, eval.HookDecisions)
}

func TestPermHook_BeforeShortCircuitsEngine(t *testing.T) {
	c := NewPermissionHookChain(&PermissionRules{Deny: []string{"Read"}})
	require.NoError(t, c.Register(PermissionHook{
		Name: "always-allow", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: overrideAllowHandle,
	}))
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, PermissionAllow, eval.FinalDecision)
	assert.True(t, eval.ShortCircuited)
	assert.Empty(t, eval.EngineDecision)
}

func TestPermHook_AfterOverridesEngine(t *testing.T) {
	c := NewPermissionHookChain(&PermissionRules{Allow: []string{"Read"}})
	require.NoError(t, c.Register(PermissionHook{
		Name: "escalate", Phase: PermissionHookAfterEvaluate,
		Enabled: true, Handle: overrideConfirmHandle,
	}))
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, PermissionAllow, eval.EngineDecision)
	assert.Equal(t, PermissionConfirm, eval.FinalDecision)
}

func TestPermHook_PriorityOrdering(t *testing.T) {
	c := NewPermissionHookChain(nil)
	calls := []string{}
	mu := sync.Mutex{}
	record := func(name string) PermissionHookHandle {
		return func(ctx context.Context, req PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
			mu.Lock()
			calls = append(calls, name)
			mu.Unlock()
			return PermissionHookContinue, ""
		}
	}
	c.Register(PermissionHook{Name: "c", Phase: PermissionHookBeforeEvaluate, Priority: 30, Enabled: true, Handle: record("c")})
	c.Register(PermissionHook{Name: "a", Phase: PermissionHookBeforeEvaluate, Priority: 10, Enabled: true, Handle: record("a")})
	c.Register(PermissionHook{Name: "b", Phase: PermissionHookBeforeEvaluate, Priority: 20, Enabled: true, Handle: record("b")})
	c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, []string{"a", "b", "c"}, calls)
}

func TestPermHook_TieBrokenByName(t *testing.T) {
	c := NewPermissionHookChain(nil)
	calls := []string{}
	mu := sync.Mutex{}
	record := func(name string) PermissionHookHandle {
		return func(ctx context.Context, req PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
			mu.Lock()
			calls = append(calls, name)
			mu.Unlock()
			return PermissionHookContinue, ""
		}
	}
	c.Register(PermissionHook{Name: "zeta", Phase: PermissionHookBeforeEvaluate, Priority: 10, Enabled: true, Handle: record("zeta")})
	c.Register(PermissionHook{Name: "alpha", Phase: PermissionHookBeforeEvaluate, Priority: 10, Enabled: true, Handle: record("alpha")})
	c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, []string{"alpha", "zeta"}, calls)
}

func TestPermHook_AuditRecordsAllHookDecisions(t *testing.T) {
	c := NewPermissionHookChain(nil)
	c.Register(PermissionHook{
		Name: "before", Phase: PermissionHookBeforeEvaluate,
		Enabled: true, Handle: continueHandle("before-pass"),
	})
	c.Register(PermissionHook{
		Name: "after", Phase: PermissionHookAfterEvaluate,
		Enabled: true, Handle: continueHandle("after-pass"),
	})
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	require.Equal(t, 2, len(eval.HookDecisions))
	assert.Equal(t, "before-pass", eval.HookDecisions[0].Reason)
	assert.Equal(t, "after-pass", eval.HookDecisions[1].Reason)
}

func TestPermHook_FirstBeforeOverrideStopsChain(t *testing.T) {
	c := NewPermissionHookChain(nil)
	c.Register(PermissionHook{
		Name: "a-deny", Phase: PermissionHookBeforeEvaluate, Priority: 10,
		Enabled: true, Handle: overrideDenyHandle,
	})
	bCalled := false
	c.Register(PermissionHook{
		Name: "b-allow", Phase: PermissionHookBeforeEvaluate, Priority: 20,
		Enabled: true, Handle: func(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
			bCalled = true
			return PermissionHookOverrideAllow, ""
		},
	})
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, PermissionDeny, eval.FinalDecision)
	assert.False(t, bCalled, "second Before hook must not run after short-circuit")
}

func TestPermHook_AfterAllHooksRunEvenAfterOverride(t *testing.T) {
	// After hooks all run; the LAST override wins.
	c := NewPermissionHookChain(&PermissionRules{Allow: []string{"Read"}})
	c.Register(PermissionHook{
		Name: "a-confirm", Phase: PermissionHookAfterEvaluate, Priority: 10,
		Enabled: true, Handle: overrideConfirmHandle,
	})
	c.Register(PermissionHook{
		Name: "b-deny", Phase: PermissionHookAfterEvaluate, Priority: 20,
		Enabled: true, Handle: overrideDenyHandle,
	})
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, PermissionDeny, eval.FinalDecision)
	assert.Equal(t, 2, len(eval.HookDecisions))
}

func TestPermHook_AfterHookSeesEngineDecision(t *testing.T) {
	seen := PermissionDecision("")
	c := NewPermissionHookChain(&PermissionRules{Deny: []string{"Bash"}})
	c.Register(PermissionHook{
		Name: "observer", Phase: PermissionHookAfterEvaluate, Enabled: true,
		Handle: func(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
			seen = e
			return PermissionHookContinue, ""
		},
	})
	c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
	assert.Equal(t, PermissionDeny, seen)
}

func TestPermHook_ConcurrentEvaluateSafe(t *testing.T) {
	c := NewPermissionHookChain(&PermissionRules{Deny: []string{"Bash"}})
	c.Register(PermissionHook{
		Name: "noop", Phase: PermissionHookAfterEvaluate, Enabled: true,
		Handle: noopHookHandle,
	})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
			assert.Equal(t, PermissionDeny, eval.FinalDecision)
		}()
	}
	wg.Wait()
}

func TestPermHook_ClockInjectableForAuditTimestamps(t *testing.T) {
	stamp := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	c := NewPermissionHookChain(nil)
	c.SetClock(func() time.Time { return stamp })
	c.Register(PermissionHook{
		Name: "x", Phase: PermissionHookBeforeEvaluate, Enabled: true,
		Handle: noopHookHandle,
	})
	eval := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Read"})
	assert.Equal(t, stamp, eval.EvaluatedAt)
	require.Equal(t, 1, len(eval.HookDecisions))
	assert.Equal(t, stamp, eval.HookDecisions[0].At)
}

func TestPermHook_SetRulesReplacesActiveRules(t *testing.T) {
	c := NewPermissionHookChain(nil)
	eval1 := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
	assert.Equal(t, PermissionAllow, eval1.FinalDecision)
	c.SetRules(&PermissionRules{Deny: []string{"Bash"}})
	eval2 := c.Evaluate(context.Background(), PermissionHookRequest{ToolName: "Bash"})
	assert.Equal(t, PermissionDeny, eval2.FinalDecision)
}

// --- helpers ---

func noopHookHandle(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
	return PermissionHookContinue, ""
}

func overrideAllowHandle(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
	return PermissionHookOverrideAllow, "force allow"
}

func overrideDenyHandle(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
	return PermissionHookOverrideDeny, "force deny"
}

func overrideConfirmHandle(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
	return PermissionHookOverrideConfirm, "escalate"
}

func continueHandle(reason string) PermissionHookHandle {
	return func(ctx context.Context, r PermissionHookRequest, e PermissionDecision) (PermissionHookOutcome, string) {
		return PermissionHookContinue, reason
	}
}
