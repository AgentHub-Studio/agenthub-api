package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EXT-003 — Hook schemas and lifecycle events.
//
// PDF arXiv:2604.14228v1 §6.1 (Hook lifecycle events with strict
// payload contracts) + §10 (auditable event envelopes).
//
// Distinct from existing hook plumbing:
//   - hook.go = AgentHook entity (per-agent registration; HOW)
//   - hooktype.go = HookCommand action variants + AllExtendedHookEvents
//     name list (WHAT events exist)
//   - hook_schema_registry.go = EVENT SCHEMA registry (WHAT each event's
//     payload must look like, for validation before dispatch)
//
// The registry exists so producers (runner/tool-executor/permission-
// gate/compaction-pipeline) emit envelopes that downstream consumers
// (hook handlers, audit sink, observability) can rely on shape-wise
// without inspecting per-call. Versioning lets the platform evolve a
// payload without breaking already-deployed consumers.

// HookSchemaVersion is an integer monotonic version. v1, v2, v3, ...
// (chosen over semver because lifecycle events have only structural
// versions — the "minor" / "patch" axes don't apply to a payload shape).
type HookSchemaVersion int

const (
	// HookSchemaVersionLatest is a sentinel meaning "current default".
	HookSchemaVersionLatest HookSchemaVersion = 0
)

// HookProducerStage identifies where in the pipeline an event originates.
// Bounded enum — used by audit sinks to filter and by tests to assert
// the producer/consumer contract.
type HookProducerStage string

const (
	HookProducerStageRunner       HookProducerStage = "runner"
	HookProducerStageToolExecutor HookProducerStage = "tool_executor"
	HookProducerStagePermission   HookProducerStage = "permission_gate"
	HookProducerStageCompaction   HookProducerStage = "compaction_pipeline"
	HookProducerStageSession      HookProducerStage = "session_manager"
	HookProducerStageSubagent     HookProducerStage = "subagent_orchestrator"
	HookProducerStageElicitation  HookProducerStage = "elicitation_loop"
)

var allHookProducerStages = []HookProducerStage{
	HookProducerStageRunner, HookProducerStageToolExecutor,
	HookProducerStagePermission, HookProducerStageCompaction,
	HookProducerStageSession, HookProducerStageSubagent,
	HookProducerStageElicitation,
}

// IsValidHookProducerStage returns true for the bounded set.
func IsValidHookProducerStage(s HookProducerStage) bool {
	for _, v := range allHookProducerStages {
		if s == v {
			return true
		}
	}
	return false
}

// AllHookProducerStages returns a copy.
func AllHookProducerStages() []HookProducerStage {
	out := make([]HookProducerStage, len(allHookProducerStages))
	copy(out, allHookProducerStages)
	return out
}

// HookSchema declares the contract for one lifecycle event at one
// version. Required keys MUST be present in any envelope's payload;
// optional keys MAY be present. Unknown keys are rejected by strict
// validation (caller can opt for lenient mode if needed).
type HookSchema struct {
	ID            uuid.UUID         `json:"id"`
	Event         ExtendedHookEvent `json:"event"`
	Version       HookSchemaVersion `json:"version"` // ≥1
	Description   string            `json:"description"`
	RequiredKeys  []string          `json:"requiredKeys"`
	OptionalKeys  []string          `json:"optionalKeys,omitempty"`
	ProducerStage HookProducerStage `json:"producerStage"`
	// Deprecated indicates this schema is still served for backward
	// compatibility but new producers must not emit at this version.
	Deprecated     bool      `json:"deprecated"`
	DeprecatedSince time.Time `json:"deprecatedSince,omitempty"`
	RegisteredAt    time.Time `json:"registeredAt"`
}

// LifecycleEventEnvelope is the standardized wrapper that all
// lifecycle event producers emit. Validation joins this against a
// registered HookSchema by (Event, Version).
type LifecycleEventEnvelope struct {
	Event         ExtendedHookEvent `json:"event"`
	Version       HookSchemaVersion `json:"version"`
	Payload       map[string]any    `json:"payload"`
	EmittedAt     time.Time         `json:"emittedAt"`
	EmittedBy     string            `json:"emittedBy"` // service/component name
	CorrelationID string            `json:"correlationId"`
}

// Sentinels.
var (
	ErrHookSchemaNotFound          = errors.New("hook schema registry: not found")
	ErrHookSchemaAlreadyRegistered = errors.New("hook schema registry: (event,version) already registered")
	ErrHookSchemaInvalidEvent      = errors.New("hook schema registry: invalid lifecycle event name")
	ErrHookSchemaInvalidStage      = errors.New("hook schema registry: invalid producer stage")
	ErrHookSchemaInvalidVersion    = errors.New("hook schema registry: version must be ≥1")
	ErrHookSchemaPayloadMissing    = errors.New("hook schema registry: payload missing required key(s)")
	ErrHookSchemaPayloadUnknown    = errors.New("hook schema registry: payload contains unknown key(s)")
	ErrHookSchemaDeprecated        = errors.New("hook schema registry: schema is deprecated")
	ErrHookSchemaRequiredKeyEmpty  = errors.New("hook schema registry: required key value cannot be empty string")
)

// HookSchemaRegistry is the persistence interface.
type HookSchemaRegistry interface {
	Register(ctx context.Context, s HookSchema) (HookSchema, error)
	Deprecate(ctx context.Context, event ExtendedHookEvent, version HookSchemaVersion) (HookSchema, error)
	Find(ctx context.Context, event ExtendedHookEvent, version HookSchemaVersion) (HookSchema, error)
	FindLatest(ctx context.Context, event ExtendedHookEvent) (HookSchema, error)
	ListVersions(ctx context.Context, event ExtendedHookEvent) ([]HookSchema, error)
	ListAll(ctx context.Context) ([]HookSchema, error)
	ListByProducerStage(ctx context.Context, stage HookProducerStage) ([]HookSchema, error)
	ValidateEnvelope(ctx context.Context, env LifecycleEventEnvelope, strict bool) error
}

// validateSchemaShape checks structural invariants.
func validateSchemaShape(s HookSchema) error {
	if !IsValidHookEvent(string(s.Event)) {
		return fmt.Errorf("%w: %q", ErrHookSchemaInvalidEvent, s.Event)
	}
	if s.Version < 1 {
		return ErrHookSchemaInvalidVersion
	}
	if !IsValidHookProducerStage(s.ProducerStage) {
		return fmt.Errorf("%w: %q", ErrHookSchemaInvalidStage, s.ProducerStage)
	}
	if strings.TrimSpace(s.Description) == "" {
		return errors.New("hook schema registry: description required")
	}
	// Required + optional must not overlap.
	req := map[string]bool{}
	for _, k := range s.RequiredKeys {
		if strings.TrimSpace(k) == "" {
			return ErrHookSchemaRequiredKeyEmpty
		}
		if req[k] {
			return fmt.Errorf("hook schema registry: required key %q duplicated", k)
		}
		req[k] = true
	}
	for _, k := range s.OptionalKeys {
		if req[k] {
			return fmt.Errorf("hook schema registry: key %q listed as both required and optional", k)
		}
	}
	return nil
}

// --- InMemoryHookSchemaRegistry ---

type schemaKey struct {
	event   ExtendedHookEvent
	version HookSchemaVersion
}

type InMemoryHookSchemaRegistry struct {
	mu      sync.Mutex
	schemas map[schemaKey]HookSchema
}

// NewInMemoryHookSchemaRegistry returns a concurrent-safe registry.
func NewInMemoryHookSchemaRegistry() *InMemoryHookSchemaRegistry {
	return &InMemoryHookSchemaRegistry{
		schemas: map[schemaKey]HookSchema{},
	}
}

// Register installs a schema for (event, version). Rejects collisions.
func (r *InMemoryHookSchemaRegistry) Register(ctx context.Context, s HookSchema) (HookSchema, error) {
	if err := ctx.Err(); err != nil {
		return HookSchema{}, err
	}
	if err := validateSchemaShape(s); err != nil {
		return HookSchema{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := schemaKey{s.Event, s.Version}
	if _, exists := r.schemas[key]; exists {
		return HookSchema{}, fmt.Errorf("%w: %q v%d", ErrHookSchemaAlreadyRegistered, s.Event, s.Version)
	}
	s.ID = uuid.New()
	s.RegisteredAt = time.Now()
	if s.Deprecated && s.DeprecatedSince.IsZero() {
		s.DeprecatedSince = s.RegisteredAt
	}
	r.schemas[key] = s
	return s, nil
}

// Deprecate marks (event, version) as deprecated. Idempotent.
func (r *InMemoryHookSchemaRegistry) Deprecate(ctx context.Context, event ExtendedHookEvent, version HookSchemaVersion) (HookSchema, error) {
	if err := ctx.Err(); err != nil {
		return HookSchema{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := schemaKey{event, version}
	s, ok := r.schemas[key]
	if !ok {
		return HookSchema{}, ErrHookSchemaNotFound
	}
	if !s.Deprecated {
		s.Deprecated = true
		s.DeprecatedSince = time.Now()
		r.schemas[key] = s
	}
	return s, nil
}

// Find returns one schema. Version=HookSchemaVersionLatest resolves to
// highest non-deprecated version (falls back to highest deprecated if all).
func (r *InMemoryHookSchemaRegistry) Find(ctx context.Context, event ExtendedHookEvent, version HookSchemaVersion) (HookSchema, error) {
	if err := ctx.Err(); err != nil {
		return HookSchema{}, err
	}
	if version == HookSchemaVersionLatest {
		return r.FindLatest(ctx, event)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.schemas[schemaKey{event, version}]
	if !ok {
		return HookSchema{}, ErrHookSchemaNotFound
	}
	return s, nil
}

// FindLatest returns the highest non-deprecated version for event.
// Falls back to highest deprecated if no live version exists.
func (r *InMemoryHookSchemaRegistry) FindLatest(ctx context.Context, event ExtendedHookEvent) (HookSchema, error) {
	if err := ctx.Err(); err != nil {
		return HookSchema{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var best HookSchema
	bestFound := false
	var fallback HookSchema
	fallbackFound := false
	for k, s := range r.schemas {
		if k.event != event {
			continue
		}
		if !s.Deprecated {
			if !bestFound || s.Version > best.Version {
				best = s
				bestFound = true
			}
		} else {
			if !fallbackFound || s.Version > fallback.Version {
				fallback = s
				fallbackFound = true
			}
		}
	}
	if bestFound {
		return best, nil
	}
	if fallbackFound {
		return fallback, nil
	}
	return HookSchema{}, ErrHookSchemaNotFound
}

// ListVersions returns all schemas for an event, sorted ascending by version.
func (r *InMemoryHookSchemaRegistry) ListVersions(ctx context.Context, event ExtendedHookEvent) ([]HookSchema, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []HookSchema{}
	for k, s := range r.schemas {
		if k.event == event {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// ListAll returns all schemas, sorted (event, version).
func (r *InMemoryHookSchemaRegistry) ListAll(ctx context.Context) ([]HookSchema, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []HookSchema{}
	for _, s := range r.schemas {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Event == out[j].Event {
			return out[i].Version < out[j].Version
		}
		return out[i].Event < out[j].Event
	})
	return out, nil
}

// ListByProducerStage returns schemas produced by stage.
func (r *InMemoryHookSchemaRegistry) ListByProducerStage(ctx context.Context, stage HookProducerStage) ([]HookSchema, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := []HookSchema{}
	for _, s := range all {
		if s.ProducerStage == stage {
			out = append(out, s)
		}
	}
	return out, nil
}

// ValidateEnvelope checks the envelope's payload against its declared
// (event, version) schema. Strict mode rejects unknown keys; lenient
// mode allows extras (forward-compat producer/older consumer).
//
// Deprecated schemas are validated normally but the caller receives a
// wrapped error so audit sinks can record the deprecation.
func (r *InMemoryHookSchemaRegistry) ValidateEnvelope(ctx context.Context, env LifecycleEventEnvelope, strict bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Event == "" {
		return errors.New("hook schema registry: envelope event required")
	}
	if env.EmittedBy == "" {
		return errors.New("hook schema registry: envelope emittedBy required")
	}
	if env.CorrelationID == "" {
		return errors.New("hook schema registry: envelope correlationId required")
	}
	if env.Version < 1 {
		return ErrHookSchemaInvalidVersion
	}
	schema, err := r.Find(ctx, env.Event, env.Version)
	if err != nil {
		return err
	}
	// Required key check.
	missing := []string{}
	for _, k := range schema.RequiredKeys {
		if _, ok := env.Payload[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%w: %s", ErrHookSchemaPayloadMissing, strings.Join(missing, ","))
	}
	// Strict-mode unknown-key rejection.
	if strict {
		allowed := map[string]bool{}
		for _, k := range schema.RequiredKeys {
			allowed[k] = true
		}
		for _, k := range schema.OptionalKeys {
			allowed[k] = true
		}
		unknown := []string{}
		for k := range env.Payload {
			if !allowed[k] {
				unknown = append(unknown, k)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return fmt.Errorf("%w: %s", ErrHookSchemaPayloadUnknown, strings.Join(unknown, ","))
		}
	}
	if schema.Deprecated {
		return fmt.Errorf("%w: %q v%d", ErrHookSchemaDeprecated, schema.Event, schema.Version)
	}
	return nil
}
