package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validDynHook() DynamicSkillHook {
	return DynamicSkillHook{
		TenantID:        "t-1",
		SkillSlug:       "web-research",
		HookSlug:        "validate-query",
		Phase:           DynamicSkillHookBeforeInvocation,
		HandlerPath:     "hooks/validate-query.yaml",
		Priority:        50,
		Enabled:         true,
		SourceExtension: "vendor/research-pack",
	}
}

func TestDynamicSkillHookPhase_EnumIsBounded(t *testing.T) {
	for _, p := range AllDynamicSkillHookPhases() {
		assert.True(t, IsValidDynamicSkillHookPhase(p))
	}
	assert.False(t, IsValidDynamicSkillHookPhase(DynamicSkillHookPhase("during")))
	assert.Equal(t, 5, len(AllDynamicSkillHookPhases()))
}

func TestDynHook_Register_AcceptsValidHook(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	saved, err := r.Register(context.Background(), validDynHook())
	require.NoError(t, err)
	assert.NotEqual(t, "", saved.ID.String())
	assert.False(t, saved.RegisteredAt.IsZero())
}

func TestDynHook_Register_RejectsMissingFields(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	for name, mutate := range map[string]func(*DynamicSkillHook){
		"tenant":  func(h *DynamicSkillHook) { h.TenantID = "" },
		"skill":   func(h *DynamicSkillHook) { h.SkillSlug = "" },
		"hook":    func(h *DynamicSkillHook) { h.HookSlug = "" },
		"handler": func(h *DynamicSkillHook) { h.HandlerPath = "" },
	} {
		h := validDynHook()
		mutate(&h)
		_, err := r.Register(context.Background(), h)
		assert.Error(t, err, "missing %s must error", name)
	}
}

func TestDynHook_Register_RejectsInvalidPhase(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	h := validDynHook()
	h.Phase = "during"
	_, err := r.Register(context.Background(), h)
	assert.True(t, errors.Is(err, ErrDynSkillHookInvalidPhase))
}

func TestDynHook_Register_RejectsDuplicate(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	_, err := r.Register(context.Background(), validDynHook())
	require.NoError(t, err)
	_, err = r.Register(context.Background(), validDynHook())
	assert.True(t, errors.Is(err, ErrDynSkillHookDuplicate))
}

func TestDynHook_Register_AllowsDifferentPhasesForSameSkillHookSlug(t *testing.T) {
	// (tenant, skill, hook_slug) is the key — differentiating by phase
	// requires using DIFFERENT hook_slug strings (caller's contract).
	r := NewInMemoryDynamicSkillHookRegistry()
	h1 := validDynHook()
	h2 := validDynHook()
	h2.HookSlug = "validate-query-after"
	h2.Phase = DynamicSkillHookAfterInvocation
	_, err := r.Register(context.Background(), h1)
	require.NoError(t, err)
	_, err = r.Register(context.Background(), h2)
	assert.NoError(t, err)
}

func TestDynHook_Register_TenantIsolation(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	_, err := r.Register(context.Background(), validDynHook())
	require.NoError(t, err)
	h2 := validDynHook()
	h2.TenantID = "t-2"
	_, err = r.Register(context.Background(), h2)
	assert.NoError(t, err, "same hook for different tenant allowed")
}

func TestDynHook_Find_RoundTrips(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	saved, _ := r.Register(context.Background(), validDynHook())
	got, err := r.Find(context.Background(), saved.TenantID, saved.SkillSlug, saved.HookSlug)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
}

func TestDynHook_Find_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	_, err := r.Find(context.Background(), "t-1", "skill", "hook")
	assert.True(t, errors.Is(err, ErrDynSkillHookNotFound))
}

func TestHooksFor_ReturnsMatchingHooksOrderedByPriorityDesc(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	for _, prio := range []int{10, 50, 30} {
		h := validDynHook()
		h.HookSlug = "hook-" + string(rune('a'+(prio/10)))
		h.Priority = prio
		_, _ = r.Register(context.Background(), h)
	}
	got, err := r.HooksFor(context.Background(), "t-1", "web-research", DynamicSkillHookBeforeInvocation)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, 50, got[0].Priority)
	assert.Equal(t, 30, got[1].Priority)
	assert.Equal(t, 10, got[2].Priority)
}

func TestHooksFor_FiltersByPhase(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	h1 := validDynHook()
	h1.Phase = DynamicSkillHookBeforeInvocation
	h2 := validDynHook()
	h2.HookSlug = "after-hook"
	h2.Phase = DynamicSkillHookAfterInvocation
	_, _ = r.Register(context.Background(), h1)
	_, _ = r.Register(context.Background(), h2)

	before, _ := r.HooksFor(context.Background(), "t-1", "web-research",
		DynamicSkillHookBeforeInvocation)
	after, _ := r.HooksFor(context.Background(), "t-1", "web-research",
		DynamicSkillHookAfterInvocation)
	assert.Len(t, before, 1)
	assert.Len(t, after, 1)
}

func TestHooksFor_ExcludesDisabledHooks(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	h := validDynHook()
	h.Enabled = false
	_, _ = r.Register(context.Background(), h)
	got, _ := r.HooksFor(context.Background(), "t-1", "web-research",
		DynamicSkillHookBeforeInvocation)
	assert.Empty(t, got)
}

func TestHooksFor_RejectsInvalidPhase(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	_, err := r.HooksFor(context.Background(), "t-1", "skill", "bogus")
	assert.True(t, errors.Is(err, ErrDynSkillHookInvalidPhase))
}

func TestHooksFor_TenantIsolation(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	_, _ = r.Register(context.Background(), validDynHook())
	got, _ := r.HooksFor(context.Background(), "t-2", "web-research",
		DynamicSkillHookBeforeInvocation)
	assert.Empty(t, got)
}

func TestListBySkill_ReturnsAllHooksForSkill(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	for _, slug := range []string{"hook-a", "hook-b", "hook-c"} {
		h := validDynHook()
		h.HookSlug = slug
		_, _ = r.Register(context.Background(), h)
	}
	got, _ := r.ListBySkill(context.Background(), "t-1", "web-research")
	assert.Len(t, got, 3)
}

func TestListBySkill_OrderedBySlug(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	for _, slug := range []string{"zeta", "alpha", "mid"} {
		h := validDynHook()
		h.HookSlug = slug
		_, _ = r.Register(context.Background(), h)
	}
	got, _ := r.ListBySkill(context.Background(), "t-1", "web-research")
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.Less(t, got[i-1].HookSlug, got[i].HookSlug)
	}
}

func TestListByExtension_FiltersBySourceExtension(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	h1 := validDynHook()
	h2 := validDynHook()
	h2.HookSlug = "other-hook"
	h2.SourceExtension = "different-vendor/pack"
	_, _ = r.Register(context.Background(), h1)
	_, _ = r.Register(context.Background(), h2)

	got, err := r.ListByExtension(context.Background(), "t-1", "vendor/research-pack")
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "validate-query", got[0].HookSlug)
}

func TestDisable_MarksDisabledIdempotent(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	saved, _ := r.Register(context.Background(), validDynHook())
	require.NoError(t, r.Disable(context.Background(), saved.TenantID, saved.SkillSlug, saved.HookSlug))
	got, _ := r.Find(context.Background(), saved.TenantID, saved.SkillSlug, saved.HookSlug)
	assert.False(t, got.Enabled)

	// Idempotent: second disable still works.
	require.NoError(t, r.Disable(context.Background(), saved.TenantID, saved.SkillSlug, saved.HookSlug))
}

func TestDisable_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	err := r.Disable(context.Background(), "t-1", "skill", "missing")
	assert.True(t, errors.Is(err, ErrDynSkillHookNotFound))
}

func TestDynHook_Delete_RemovesHook(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	saved, _ := r.Register(context.Background(), validDynHook())
	require.NoError(t, r.Delete(context.Background(), saved.TenantID, saved.SkillSlug, saved.HookSlug))
	_, err := r.Find(context.Background(), saved.TenantID, saved.SkillSlug, saved.HookSlug)
	assert.True(t, errors.Is(err, ErrDynSkillHookNotFound))
}

func TestDynHook_Delete_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	err := r.Delete(context.Background(), "t-1", "skill", "missing")
	assert.True(t, errors.Is(err, ErrDynSkillHookNotFound))
}

func TestClearExtension_RemovesAllForExtension(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	for _, slug := range []string{"a", "b", "c"} {
		h := validDynHook()
		h.HookSlug = slug
		_, _ = r.Register(context.Background(), h)
	}
	other := validDynHook()
	other.HookSlug = "other"
	other.SourceExtension = "different/pack"
	_, _ = r.Register(context.Background(), other)

	count, err := r.ClearExtension(context.Background(), "t-1", "vendor/research-pack")
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	remaining, _ := r.ListBySkill(context.Background(), "t-1", "web-research")
	assert.Len(t, remaining, 1, "only different/pack hook remains")
}

func TestDynHook_Register_ConcurrentSafe(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			h := validDynHook()
			h.HookSlug = "hook-" + string(rune('a'+(i%26)))
			_, _ = r.Register(context.Background(), h)
		}()
	}
	wg.Wait()
}

func TestDynSkillHook_ContextCancelled(t *testing.T) {
	r := NewInMemoryDynamicSkillHookRegistry()
	saved, _ := r.Register(context.Background(), validDynHook())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Register(ctx, validDynHook())
	assert.Error(t, err)
	_, err = r.Find(ctx, saved.TenantID, saved.SkillSlug, saved.HookSlug)
	assert.Error(t, err)
	_, err = r.HooksFor(ctx, saved.TenantID, saved.SkillSlug, DynamicSkillHookBeforeInvocation)
	assert.Error(t, err)
	_, err = r.ListBySkill(ctx, saved.TenantID, saved.SkillSlug)
	assert.Error(t, err)
	_, err = r.ListByExtension(ctx, saved.TenantID, saved.SourceExtension)
	assert.Error(t, err)
	err = r.Disable(ctx, saved.TenantID, saved.SkillSlug, saved.HookSlug)
	assert.Error(t, err)
	err = r.Delete(ctx, saved.TenantID, saved.SkillSlug, saved.HookSlug)
	assert.Error(t, err)
	_, err = r.ClearExtension(ctx, saved.TenantID, saved.SourceExtension)
	assert.Error(t, err)
}
