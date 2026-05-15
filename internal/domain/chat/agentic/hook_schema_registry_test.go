package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validHookSchema() HookSchema {
	return HookSchema{
		Event:         HookPreToolUseExt,
		Version:       1,
		Description:   "Fired before any tool execution; payload carries tool slug + args.",
		RequiredKeys:  []string{"tool_slug", "args"},
		OptionalKeys:  []string{"caller_role"},
		ProducerStage: HookProducerStageRunner,
	}
}

func validHookEnvelope() LifecycleEventEnvelope {
	return LifecycleEventEnvelope{
		Event:         HookPreToolUseExt,
		Version:       1,
		Payload:       map[string]any{"tool_slug": "core-list-agents", "args": map[string]any{}},
		EmittedBy:     "runner",
		CorrelationID: "corr-1",
	}
}

func TestHookSchema_ProducerStageEnumIsBounded(t *testing.T) {
	for _, s := range AllHookProducerStages() {
		assert.True(t, IsValidHookProducerStage(s))
	}
	assert.False(t, IsValidHookProducerStage(HookProducerStage("bogus")))
	assert.Equal(t, 7, len(AllHookProducerStages()))
}

func TestHookSchema_Register_AssignsIDAndTimestamp(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	saved, err := r.Register(context.Background(), validHookSchema())
	require.NoError(t, err)
	assert.NotEqual(t, "", saved.ID.String())
	assert.False(t, saved.RegisteredAt.IsZero())
}

func TestHookSchema_Register_RejectsInvalidEventName(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s := validHookSchema()
	s.Event = "NotARealEvent"
	_, err := r.Register(context.Background(), s)
	assert.True(t, errors.Is(err, ErrHookSchemaInvalidEvent))
}

func TestHookSchema_Register_RejectsInvalidProducerStage(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s := validHookSchema()
	s.ProducerStage = HookProducerStage("bogus")
	_, err := r.Register(context.Background(), s)
	assert.True(t, errors.Is(err, ErrHookSchemaInvalidStage))
}

func TestHookSchema_Register_RejectsZeroVersion(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s := validHookSchema()
	s.Version = 0
	_, err := r.Register(context.Background(), s)
	assert.True(t, errors.Is(err, ErrHookSchemaInvalidVersion))
}

func TestHookSchema_Register_RejectsDuplicateEventVersion(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, err := r.Register(context.Background(), validHookSchema())
	require.NoError(t, err)
	_, err = r.Register(context.Background(), validHookSchema())
	assert.True(t, errors.Is(err, ErrHookSchemaAlreadyRegistered))
}

func TestHookSchema_Register_AllowsMultipleVersionsOfSameEvent(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, err := r.Register(context.Background(), validHookSchema())
	require.NoError(t, err)
	v2 := validHookSchema()
	v2.Version = 2
	v2.RequiredKeys = []string{"tool_slug", "args", "caller_id"} // expanded contract
	_, err = r.Register(context.Background(), v2)
	assert.NoError(t, err)
}

func TestHookSchema_Register_RejectsEmptyDescription(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s := validHookSchema()
	s.Description = "  "
	_, err := r.Register(context.Background(), s)
	assert.Error(t, err)
}

func TestHookSchema_Register_RejectsEmptyRequiredKey(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s := validHookSchema()
	s.RequiredKeys = []string{"tool_slug", ""}
	_, err := r.Register(context.Background(), s)
	assert.True(t, errors.Is(err, ErrHookSchemaRequiredKeyEmpty))
}

func TestHookSchema_Register_RejectsDuplicateRequiredKey(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s := validHookSchema()
	s.RequiredKeys = []string{"tool_slug", "tool_slug"}
	_, err := r.Register(context.Background(), s)
	assert.Error(t, err)
}

func TestHookSchema_Register_RejectsKeyListedRequiredAndOptional(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s := validHookSchema()
	s.RequiredKeys = []string{"tool_slug"}
	s.OptionalKeys = []string{"tool_slug"} // collision
	_, err := r.Register(context.Background(), s)
	assert.Error(t, err)
}

func TestHookSchema_Find_RoundTrips(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	got, err := r.Find(context.Background(), HookPreToolUseExt, 1)
	require.NoError(t, err)
	assert.Equal(t, HookPreToolUseExt, got.Event)
}

func TestHookSchema_Find_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, err := r.Find(context.Background(), HookPreToolUseExt, 1)
	assert.True(t, errors.Is(err, ErrHookSchemaNotFound))
}

func TestHookSchema_Find_LatestSentinelReturnsHighestNonDeprecated(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	v2 := validHookSchema()
	v2.Version = 2
	_, _ = r.Register(context.Background(), v2)
	got, err := r.Find(context.Background(), HookPreToolUseExt, HookSchemaVersionLatest)
	require.NoError(t, err)
	assert.Equal(t, HookSchemaVersion(2), got.Version)
}

func TestHookSchema_FindLatest_PrefersNonDeprecatedOverDeprecated(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	v1 := validHookSchema()
	v1.Version = 1
	_, _ = r.Register(context.Background(), v1)

	v2 := validHookSchema()
	v2.Version = 2
	_, _ = r.Register(context.Background(), v2)

	// Deprecate v2 → FindLatest should fall back to v1.
	_, _ = r.Deprecate(context.Background(), HookPreToolUseExt, 2)
	got, err := r.FindLatest(context.Background(), HookPreToolUseExt)
	require.NoError(t, err)
	assert.Equal(t, HookSchemaVersion(1), got.Version,
		"latest must prefer non-deprecated over deprecated")
}

func TestHookSchema_FindLatest_FallsBackToDeprecatedIfNoLive(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	_, _ = r.Deprecate(context.Background(), HookPreToolUseExt, 1)
	got, err := r.FindLatest(context.Background(), HookPreToolUseExt)
	require.NoError(t, err)
	assert.Equal(t, HookSchemaVersion(1), got.Version)
	assert.True(t, got.Deprecated)
}

func TestHookSchema_Deprecate_IsIdempotent(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	first, err := r.Deprecate(context.Background(), HookPreToolUseExt, 1)
	require.NoError(t, err)
	second, err := r.Deprecate(context.Background(), HookPreToolUseExt, 1)
	require.NoError(t, err)
	assert.Equal(t, first.DeprecatedSince, second.DeprecatedSince,
		"second deprecate must not bump timestamp")
}

func TestHookSchema_Deprecate_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, err := r.Deprecate(context.Background(), HookPreToolUseExt, 1)
	assert.True(t, errors.Is(err, ErrHookSchemaNotFound))
}

func TestHookSchema_ListVersions_AscendingOrder(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	for _, v := range []HookSchemaVersion{3, 1, 2} {
		s := validHookSchema()
		s.Version = v
		_, _ = r.Register(context.Background(), s)
	}
	got, err := r.ListVersions(context.Background(), HookPreToolUseExt)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.Less(t, got[i-1].Version, got[i].Version)
	}
}

func TestHookSchema_ListAll_OrderedByEventThenVersion(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s1 := validHookSchema()
	s1.Event = HookSessionStartExt
	_, _ = r.Register(context.Background(), s1)
	s2 := validHookSchema()
	s2.Event = HookPreToolUseExt
	_, _ = r.Register(context.Background(), s2)
	all, err := r.ListAll(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 2)
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, string(all[i-1].Event), string(all[i].Event))
	}
}

func TestHookSchema_ListByProducerStage_Filters(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	s1 := validHookSchema()
	s1.Event = HookPreToolUseExt
	_, _ = r.Register(context.Background(), s1)

	s2 := validHookSchema()
	s2.Event = HookPermissionRequestExt
	s2.ProducerStage = HookProducerStagePermission
	_, _ = r.Register(context.Background(), s2)

	got, err := r.ListByProducerStage(context.Background(), HookProducerStagePermission)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, HookPermissionRequestExt, got[0].Event)
}

func TestHookSchema_ValidateEnvelope_PassesOnExactMatch(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	err := r.ValidateEnvelope(context.Background(), validHookEnvelope(), true)
	assert.NoError(t, err)
}

func TestHookSchema_ValidateEnvelope_FailsOnMissingRequired(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	env := validHookEnvelope()
	delete(env.Payload, "args")
	err := r.ValidateEnvelope(context.Background(), env, false)
	assert.True(t, errors.Is(err, ErrHookSchemaPayloadMissing))
}

func TestHookSchema_ValidateEnvelope_StrictRejectsUnknownKeys(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	env := validHookEnvelope()
	env.Payload["surprise_key"] = "x"
	err := r.ValidateEnvelope(context.Background(), env, true)
	assert.True(t, errors.Is(err, ErrHookSchemaPayloadUnknown))
}

func TestHookSchema_ValidateEnvelope_LenientAllowsUnknownKeys(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	env := validHookEnvelope()
	env.Payload["forward_compat_key"] = "x"
	err := r.ValidateEnvelope(context.Background(), env, false)
	assert.NoError(t, err, "lenient mode must allow forward-compat extras")
}

func TestHookSchema_ValidateEnvelope_RejectsEmptyEmittedBy(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	env := validHookEnvelope()
	env.EmittedBy = ""
	err := r.ValidateEnvelope(context.Background(), env, false)
	assert.Error(t, err)
}

func TestHookSchema_ValidateEnvelope_RejectsEmptyCorrelationID(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	env := validHookEnvelope()
	env.CorrelationID = ""
	err := r.ValidateEnvelope(context.Background(), env, false)
	assert.Error(t, err)
}

func TestHookSchema_ValidateEnvelope_RejectsUnknownVersion(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	env := validHookEnvelope()
	env.Version = 99
	err := r.ValidateEnvelope(context.Background(), env, false)
	assert.True(t, errors.Is(err, ErrHookSchemaNotFound))
}

func TestHookSchema_ValidateEnvelope_DeprecatedSchemaSurfacesError(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	_, _ = r.Deprecate(context.Background(), HookPreToolUseExt, 1)
	err := r.ValidateEnvelope(context.Background(), validHookEnvelope(), false)
	assert.True(t, errors.Is(err, ErrHookSchemaDeprecated),
		"deprecated must surface so audit sink records the deprecation")
}

func TestHookSchema_ConcurrentRegisterIsSafe(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := validHookSchema()
			s.Version = HookSchemaVersion(i + 1)
			_, _ = r.Register(context.Background(), s)
		}()
	}
	wg.Wait()
	all, _ := r.ListAll(context.Background())
	assert.Equal(t, 50, len(all))
}

func TestHookSchema_ContextCancelled(t *testing.T) {
	r := NewInMemoryHookSchemaRegistry()
	_, _ = r.Register(context.Background(), validHookSchema())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Register(ctx, validHookSchema())
	assert.Error(t, err)
	_, err = r.Find(ctx, HookPreToolUseExt, 1)
	assert.Error(t, err)
	_, err = r.FindLatest(ctx, HookPreToolUseExt)
	assert.Error(t, err)
	_, err = r.Deprecate(ctx, HookPreToolUseExt, 1)
	assert.Error(t, err)
	_, err = r.ListVersions(ctx, HookPreToolUseExt)
	assert.Error(t, err)
	_, err = r.ListAll(ctx)
	assert.Error(t, err)
	_, err = r.ListByProducerStage(ctx, HookProducerStageRunner)
	assert.Error(t, err)
	err = r.ValidateEnvelope(ctx, validHookEnvelope(), false)
	assert.Error(t, err)
}
