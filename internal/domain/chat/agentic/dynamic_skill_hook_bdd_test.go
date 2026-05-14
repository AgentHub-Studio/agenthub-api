package agentic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_DynamicSkillHook(t *testing.T) {
	t.Run("Scenario_SkillRegistersValidatorAtRegistrationTime", func(t *testing.T) {
		// Given a research-pack skill ships with a query validator,
		// And the validator should run before the skill body executes,
		// When the extension installer registers the hook,
		// Then HooksFor("research-pack", before_invocation) returns it.
		r := NewInMemoryDynamicSkillHookRegistry()
		_, err := r.Register(context.Background(), validDynHook())
		require.NoError(t, err)
		got, _ := r.HooksFor(context.Background(), "t-1", "web-research",
			DynamicSkillHookBeforeInvocation)
		require.Len(t, got, 1)
		assert.Equal(t, "validate-query", got[0].HookSlug)
	})

	t.Run("Scenario_HooksFireOnlyForOwnSkillNotOthers", func(t *testing.T) {
		// Given skill-A has its own validator,
		// When skill-B is invoked,
		// Then skill-A's hook does NOT fire (per-skill scope, no global
		// pollution).
		r := NewInMemoryDynamicSkillHookRegistry()
		_, _ = r.Register(context.Background(), validDynHook())
		got, _ := r.HooksFor(context.Background(), "t-1", "different-skill",
			DynamicSkillHookBeforeInvocation)
		assert.Empty(t, got)
	})

	t.Run("Scenario_PriorityOrdersHookExecutionForDeterministicReplay", func(t *testing.T) {
		// Given multiple hooks fire on the same phase,
		// When the runtime fetches them,
		// Then they come back priority-desc — replay debugging sees the
		// same execution order each time.
		r := NewInMemoryDynamicSkillHookRegistry()
		for _, prio := range []int{10, 90, 50} {
			h := validDynHook()
			h.HookSlug = "prio-" + string(rune('a'+(prio/10)))
			h.Priority = prio
			_, _ = r.Register(context.Background(), h)
		}
		got, _ := r.HooksFor(context.Background(), "t-1", "web-research",
			DynamicSkillHookBeforeInvocation)
		require.Len(t, got, 3)
		assert.Equal(t, 90, got[0].Priority)
		assert.Equal(t, 50, got[1].Priority)
		assert.Equal(t, 10, got[2].Priority)
	})

	t.Run("Scenario_DisabledHooksExcludedFromExecutionWithoutDelete", func(t *testing.T) {
		// Given admin wants to temporarily silence a hook for debugging
		// without losing its registration,
		// When admin disables it,
		// Then HooksFor excludes it but Find still returns the record.
		r := NewInMemoryDynamicSkillHookRegistry()
		saved, _ := r.Register(context.Background(), validDynHook())
		require.NoError(t, r.Disable(context.Background(), saved.TenantID,
			saved.SkillSlug, saved.HookSlug))

		runtime, _ := r.HooksFor(context.Background(), "t-1", "web-research",
			DynamicSkillHookBeforeInvocation)
		assert.Empty(t, runtime, "disabled hook excluded from runtime")

		stored, _ := r.Find(context.Background(), "t-1", "web-research", "validate-query")
		assert.False(t, stored.Enabled, "but record persists for re-enable")
	})

	t.Run("Scenario_ExtensionUninstallClearsAllItsHooksAtomically", func(t *testing.T) {
		// Given an extension registered 5 dynamic hooks across 3 skills,
		// When admin uninstalls the extension,
		// Then ClearExtension removes ALL of them atomically (no orphan
		// hooks pointing at uninstalled extensions).
		r := NewInMemoryDynamicSkillHookRegistry()
		for _, hookSlug := range []string{"a", "b", "c", "d", "e"} {
			h := validDynHook()
			h.HookSlug = hookSlug
			_, _ = r.Register(context.Background(), h)
		}
		// Different extension should NOT be cleared.
		other := validDynHook()
		other.HookSlug = "kept"
		other.SourceExtension = "different/pack"
		_, _ = r.Register(context.Background(), other)

		count, _ := r.ClearExtension(context.Background(), "t-1", "vendor/research-pack")
		assert.Equal(t, 5, count)

		remaining, _ := r.ListBySkill(context.Background(), "t-1", "web-research")
		require.Len(t, remaining, 1)
		assert.Equal(t, "kept", remaining[0].HookSlug)
	})

	t.Run("Scenario_FivePhasesCoverSkillLifecycle", func(t *testing.T) {
		// Given the skill lifecycle has 5 distinct moments to observe,
		// When the bounded enum is queried,
		// Then exactly 5 phases exist (before/after invocation,
		// before/after tool call, on error).
		assert.Equal(t, 5, len(AllDynamicSkillHookPhases()))
	})

	t.Run("Scenario_DuplicateHookSlugSameSkillRejectedToPreventOverwrite", func(t *testing.T) {
		// Given (tenant, skill, hook_slug) is the permanent key,
		// When admin re-registers the same key,
		// Then registry rejects — must Delete first to update (explicit
		// lifecycle, no silent overwrite).
		r := NewInMemoryDynamicSkillHookRegistry()
		_, _ = r.Register(context.Background(), validDynHook())
		_, err := r.Register(context.Background(), validDynHook())
		assert.True(t, errors.Is(err, ErrDynSkillHookDuplicate))
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantHookBleed", func(t *testing.T) {
		// Given tenant-A has a custom validator hook on skill X,
		// When tenant-B invokes skill X,
		// Then tenant-B's HooksFor does NOT include tenant-A's hook
		// (per-tenant index keying).
		r := NewInMemoryDynamicSkillHookRegistry()
		_, _ = r.Register(context.Background(), validDynHook())
		got, _ := r.HooksFor(context.Background(), "t-2", "web-research",
			DynamicSkillHookBeforeInvocation)
		assert.Empty(t, got)
	})

	t.Run("Scenario_ListByExtensionEnablesUninstallPreviewForAdmin", func(t *testing.T) {
		// Given admin previews uninstall ("what hooks does this extension
		// own?"),
		// When ListByExtension runs,
		// Then admin sees every hook this extension would lose — drives
		// confirmation UI before actual uninstall.
		r := NewInMemoryDynamicSkillHookRegistry()
		_, _ = r.Register(context.Background(), validDynHook())
		got, _ := r.ListByExtension(context.Background(), "t-1", "vendor/research-pack")
		require.Len(t, got, 1)
		assert.Equal(t, "validate-query", got[0].HookSlug)
	})

	t.Run("Scenario_HookHandlerPathIsPrintedForGOV001AuditOnFire", func(t *testing.T) {
		// Given GOV-001 audits which handler ran for which event,
		// When HooksFor returns,
		// Then HandlerPath identifies the entry — auditor can reproduce.
		r := NewInMemoryDynamicSkillHookRegistry()
		_, _ = r.Register(context.Background(), validDynHook())
		got, _ := r.HooksFor(context.Background(), "t-1", "web-research",
			DynamicSkillHookBeforeInvocation)
		require.Len(t, got, 1)
		assert.NotEmpty(t, got[0].HandlerPath)
	})
}
