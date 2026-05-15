package agentic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_HookSchemaRegistry(t *testing.T) {
	t.Run("Scenario_ProducerEmitsEnvelopeAndConsumerValidatesAgainstSchema", func(t *testing.T) {
		// Given the runner declares PreToolUse v1 schema (tool_slug +
		// args required),
		// When the runner emits a PreToolUse envelope with both fields,
		// Then the audit sink validates it against the schema and accepts
		// without inspecting individual fields per call.
		r := NewInMemoryHookSchemaRegistry()
		_, err := r.Register(context.Background(), validHookSchema())
		require.NoError(t, err)
		err = r.ValidateEnvelope(context.Background(), validHookEnvelope(), true)
		assert.NoError(t, err)
	})

	t.Run("Scenario_ProducerOmitsRequiredFieldAndIsRejectedAtBoundary", func(t *testing.T) {
		// Given a buggy producer omits the args field,
		// When the envelope arrives at the validation boundary,
		// Then the registry rejects it (catches drift before consumers
		// crash on missing keys downstream).
		r := NewInMemoryHookSchemaRegistry()
		_, _ = r.Register(context.Background(), validHookSchema())
		env := validHookEnvelope()
		delete(env.Payload, "args")
		err := r.ValidateEnvelope(context.Background(), env, false)
		assert.Error(t, err)
	})

	t.Run("Scenario_StrictModeRejectsUnknownKeysToCatchDriftEarly", func(t *testing.T) {
		// Given strict-mode validation in CI,
		// When a producer adds an undocumented key,
		// Then strict validation flags it so the team adds the key to
		// the schema (or removes it) before merging.
		r := NewInMemoryHookSchemaRegistry()
		_, _ = r.Register(context.Background(), validHookSchema())
		env := validHookEnvelope()
		env.Payload["new_metric_count"] = 5
		err := r.ValidateEnvelope(context.Background(), env, true)
		assert.True(t, errors.Is(err, ErrHookSchemaPayloadUnknown))
	})

	t.Run("Scenario_LenientModeAllowsForwardCompatibleConsumers", func(t *testing.T) {
		// Given a consumer is on the older schema version while a
		// producer has rolled out a newer field,
		// When validation runs in lenient mode (production hot path),
		// Then the extra forward-compat key passes (consumer ignores it
		// gracefully; rollout staggered).
		r := NewInMemoryHookSchemaRegistry()
		_, _ = r.Register(context.Background(), validHookSchema())
		env := validHookEnvelope()
		env.Payload["forward_compat_field"] = "ok"
		err := r.ValidateEnvelope(context.Background(), env, false)
		assert.NoError(t, err)
	})

	t.Run("Scenario_MultiVersionCoexistenceLetsRolloutBeStaggered", func(t *testing.T) {
		// Given the platform supports v1 and v2 of PreToolUse simultaneously,
		// When v2 producer emits while v1 consumer still active,
		// Then both versions resolve to their own schema and validate
		// independently (no big-bang version cutover).
		r := NewInMemoryHookSchemaRegistry()
		v1 := validHookSchema()
		_, _ = r.Register(context.Background(), v1)
		v2 := validHookSchema()
		v2.Version = 2
		v2.RequiredKeys = []string{"tool_slug", "args", "caller_id"}
		_, _ = r.Register(context.Background(), v2)

		env1 := validHookEnvelope()
		env1.Version = 1
		assert.NoError(t, r.ValidateEnvelope(context.Background(), env1, false))

		env2 := validHookEnvelope()
		env2.Version = 2
		env2.Payload["caller_id"] = "alice"
		assert.NoError(t, r.ValidateEnvelope(context.Background(), env2, false))
	})

	t.Run("Scenario_DeprecationSurfacesAuditSignalWithoutBreakingProducers", func(t *testing.T) {
		// Given v1 is being phased out and the team marks it deprecated,
		// When v1 producers continue emitting (gradual rollout),
		// Then validation still succeeds structurally but returns the
		// deprecated sentinel — audit sink records "this producer is
		// behind" so the team can drive the rollout.
		r := NewInMemoryHookSchemaRegistry()
		_, _ = r.Register(context.Background(), validHookSchema())
		_, _ = r.Deprecate(context.Background(), HookPreToolUseExt, 1)
		err := r.ValidateEnvelope(context.Background(), validHookEnvelope(), false)
		assert.True(t, errors.Is(err, ErrHookSchemaDeprecated))
	})

	t.Run("Scenario_FindLatestPrefersLiveOverDeprecated", func(t *testing.T) {
		// Given v1 is deprecated and v2 is live,
		// When a new producer asks "which version should I emit",
		// Then FindLatest returns v2 (never deprecated) so new code
		// jumps directly to the current contract.
		r := NewInMemoryHookSchemaRegistry()
		v1 := validHookSchema()
		_, _ = r.Register(context.Background(), v1)
		v2 := validHookSchema()
		v2.Version = 2
		_, _ = r.Register(context.Background(), v2)
		_, _ = r.Deprecate(context.Background(), HookPreToolUseExt, 1)
		latest, err := r.FindLatest(context.Background(), HookPreToolUseExt)
		require.NoError(t, err)
		assert.Equal(t, HookSchemaVersion(2), latest.Version)
		assert.False(t, latest.Deprecated)
	})

	t.Run("Scenario_AdminInspectsEventsByProducerForOnboardingDocs", func(t *testing.T) {
		// Given onboarding docs need to list "events the runner emits"
		// vs "events the permission gate emits",
		// When the docs builder calls ListByProducerStage,
		// Then it gets the schemas grouped by their producing component
		// (no manual scanning of every schema).
		r := NewInMemoryHookSchemaRegistry()
		s1 := validHookSchema()
		_, _ = r.Register(context.Background(), s1)

		s2 := validHookSchema()
		s2.Event = HookPermissionRequestExt
		s2.ProducerStage = HookProducerStagePermission
		s2.RequiredKeys = []string{"requested_action", "actor"}
		_, _ = r.Register(context.Background(), s2)

		runner, err := r.ListByProducerStage(context.Background(), HookProducerStageRunner)
		require.NoError(t, err)
		assert.Len(t, runner, 1)
		assert.Equal(t, HookPreToolUseExt, runner[0].Event)
	})

	t.Run("Scenario_EnvelopeRequiresCorrelationIDForDistributedTracing", func(t *testing.T) {
		// Given OBS-002 distributed tracing joins envelopes by
		// correlationId across producer→consumer chains,
		// When a producer forgets the correlationId,
		// Then validation rejects (no untraceable events on the wire).
		r := NewInMemoryHookSchemaRegistry()
		_, _ = r.Register(context.Background(), validHookSchema())
		env := validHookEnvelope()
		env.CorrelationID = ""
		err := r.ValidateEnvelope(context.Background(), env, false)
		assert.Error(t, err)
	})

	t.Run("Scenario_RegistryIsSourceOfTruthForAllowedEventNames", func(t *testing.T) {
		// Given the platform tightly bounds what counts as a lifecycle event,
		// When a producer tries to register a schema for an unknown event,
		// Then registration fails (no rogue event names polluting the wire).
		r := NewInMemoryHookSchemaRegistry()
		s := validHookSchema()
		s.Event = ExtendedHookEvent("MyCustomEvent")
		_, err := r.Register(context.Background(), s)
		assert.True(t, errors.Is(err, ErrHookSchemaInvalidEvent))
	})
}
