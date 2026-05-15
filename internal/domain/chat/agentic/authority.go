package agentic

import (
	"errors"
	"fmt"
	"sort"
)

// GOV-006 — Authority hierarchy.
//
// PDF arXiv:2604.14228v1 §11 (governance — enterprise > tenant > user
// settings layering with lock semantics); §6.1 (settings is one of 10
// plugin manifest types); CLAUDE.md (multi-tenant + per-agent overrides).
//
// Existing in AgentHub:
//   - platform_settings (ah_core, lowest authority — defaults)
//   - tenant_settings (per-tenant)
//   - agent.model_config (per-agent)
//   - user settings (per-user)
//
// What was missing: a UNIFIED RESOLVER that collapses multiple layers
// into one effective value AND enforces lock semantics (a higher
// authority can mark a setting "locked" — lower layers cannot override).
//
// Resolution rules:
//   1. Layers are ordered low→high by authority level (platform=0 → runtime=5).
//   2. Higher layers override lower layers' VALUES.
//   3. If ANY layer marks a key as locked, NO lower-OR-EQUAL layer can change it
//      after the lock — but HIGHER layers can still override (admin escalation).
//   4. Wait — that's the wrong policy. Correct: a lock by layer N freezes the
//      value for ALL layers > N too. This is the "platform sets baseline,
//      no one below or above can change it" semantics — used for security
//      baselines like audit_retention_days.

// AuthorityLevel bounded enum, ordered low→high. Higher levels normally
// override lower ones (last-writer-wins). Locks (see AuthorityLayer.Locked)
// flip this for specific keys.
type AuthorityLevel int

const (
	// AuthorityPlatform — ah_core platform defaults (lowest, applies to everyone).
	AuthorityPlatform AuthorityLevel = 0
	// AuthorityEnterprise — enterprise/customer-org-wide overrides.
	AuthorityEnterprise AuthorityLevel = 1
	// AuthorityTenant — per-tenant overrides.
	AuthorityTenant AuthorityLevel = 2
	// AuthorityAgent — per-agent runtime config.
	AuthorityAgent AuthorityLevel = 3
	// AuthorityUser — per-user preferences.
	AuthorityUser AuthorityLevel = 4
	// AuthorityRuntime — per-request override (transient, e.g. temperature
	// passed in the API call). Highest authority.
	AuthorityRuntime AuthorityLevel = 5
)

// allAuthorityLevels is the closed bounded set in low→high order.
var allAuthorityLevels = []AuthorityLevel{
	AuthorityPlatform,
	AuthorityEnterprise,
	AuthorityTenant,
	AuthorityAgent,
	AuthorityUser,
	AuthorityRuntime,
}

// IsValidAuthorityLevel returns true for the bounded set.
func IsValidAuthorityLevel(l AuthorityLevel) bool {
	for _, v := range allAuthorityLevels {
		if l == v {
			return true
		}
	}
	return false
}

// AllAuthorityLevels returns a copy of the bounded set in low→high order.
func AllAuthorityLevels() []AuthorityLevel {
	out := make([]AuthorityLevel, len(allAuthorityLevels))
	copy(out, allAuthorityLevels)
	return out
}

// String returns the stable string representation. Used in audit logs +
// EffectiveSetting.SourceLevel attribution.
func (l AuthorityLevel) String() string {
	switch l {
	case AuthorityPlatform:
		return "platform"
	case AuthorityEnterprise:
		return "enterprise"
	case AuthorityTenant:
		return "tenant"
	case AuthorityAgent:
		return "agent"
	case AuthorityUser:
		return "user"
	case AuthorityRuntime:
		return "runtime"
	}
	return "unknown"
}

// AuthorityLayer is one layer in the resolution chain.
type AuthorityLayer struct {
	// Level positions this layer in the hierarchy. Required.
	Level AuthorityLevel
	// ScopeID identifies the concrete entity (tenant ID, agent ID, etc.)
	// for audit attribution. Optional but recommended.
	ScopeID string
	// Settings is the key→value bag this layer contributes.
	Settings map[string]string
	// Locked is the set of keys this layer LOCKS — once set, no other
	// layer (higher OR lower) can override the value.
	// Maps key → reason (audit-friendly).
	Locked map[string]string
}

// EffectiveSetting is the resolved value plus source attribution.
type EffectiveSetting struct {
	// Key is the setting identifier.
	Key string
	// Value is the resolved value (after applying all layers + locks).
	Value string
	// SourceLevel identifies which authority layer produced this value.
	SourceLevel AuthorityLevel
	// SourceScopeID is the ScopeID of the source layer.
	SourceScopeID string
	// IsLocked is true when SOME layer has locked this key.
	IsLocked bool
	// LockReason carries the reason from the locking layer (when IsLocked).
	LockReason string
	// LockSourceLevel identifies which layer locked the key.
	LockSourceLevel AuthorityLevel
}

// ErrInvalidAuthorityLevel — defensive guard for bad inputs.
var ErrInvalidAuthorityLevel = errors.New("authority: invalid level")

// ErrLockedKeyOverrideAttempt — returned by Resolve when integrity_check=true
// and a layer attempts to set a key already locked by a different layer.
var ErrLockedKeyOverrideAttempt = errors.New("authority: locked key override attempted")

// LayeredResolver resolves multiple AuthorityLayer into per-key EffectiveSettings.
type LayeredResolver struct {
	// IntegrityCheck: when true, Resolve errors if any layer attempts to
	// set a key that another layer has locked. Default false (silent
	// drop with audit attribution to lock source).
	IntegrityCheck bool
}

// NewLayeredResolver creates a resolver with default config (no integrity check).
func NewLayeredResolver() *LayeredResolver {
	return &LayeredResolver{}
}

// WithIntegrityCheck flips the IntegrityCheck flag and returns the
// resolver for chaining.
func (r *LayeredResolver) WithIntegrityCheck() *LayeredResolver {
	r.IntegrityCheck = true
	return r
}

// Resolve collapses the given layers into a map of EffectiveSettings.
//
// Semantics:
//   1. Layers sorted low→high by Level (stable order on ties — input order).
//   2. For each key across all layers: apply settings in order; later layers
//      override earlier ones — UNLESS a lock blocks it.
//   3. A lock by layer L freezes the value at that layer's Settings[key]
//      (or, if L doesn't set it, at whatever value was current when L
//      ran — usually the platform default).
//   4. Lock attribution wins by FIRST-LOCK (the earliest layer to lock a
//      key is the source of truth; later locks for the same key are
//      ignored — locks are first-write-wins, like checkpoint resolutions).
func (r *LayeredResolver) Resolve(layers []AuthorityLayer) (map[string]EffectiveSetting, error) {
	for _, l := range layers {
		if !IsValidAuthorityLevel(l.Level) {
			return nil, fmt.Errorf("%w: %d", ErrInvalidAuthorityLevel, l.Level)
		}
	}

	// Stable sort by Level (preserve input order on ties).
	sortedLayers := make([]AuthorityLayer, len(layers))
	copy(sortedLayers, layers)
	sort.SliceStable(sortedLayers, func(i, j int) bool {
		return sortedLayers[i].Level < sortedLayers[j].Level
	})

	effective := map[string]EffectiveSetting{}

	// Pass 1: apply settings + record locks. Walk low→high.
	for _, layer := range sortedLayers {
		// First, apply all settings in this layer (later wins by default).
		for k, v := range layer.Settings {
			cur, exists := effective[k]
			if exists && cur.IsLocked {
				if r.IntegrityCheck {
					return nil, fmt.Errorf("%w: layer %s tried to override key %q locked by %s",
						ErrLockedKeyOverrideAttempt,
						layer.Level.String(), k, cur.LockSourceLevel.String())
				}
				// Silent drop — locked value persists.
				continue
			}
			effective[k] = EffectiveSetting{
				Key:           k,
				Value:         v,
				SourceLevel:   layer.Level,
				SourceScopeID: layer.ScopeID,
				IsLocked:      cur.IsLocked,
				LockReason:    cur.LockReason,
				LockSourceLevel: cur.LockSourceLevel,
			}
		}

		// Then, register locks from this layer. First-lock-wins per key.
		for k, reason := range layer.Locked {
			cur, exists := effective[k]
			if !exists {
				// Lock with no value yet — record an empty effective whose
				// source is the lock itself (lower-precedence callers
				// understand this means "explicitly locked-empty").
				effective[k] = EffectiveSetting{
					Key:             k,
					Value:           "",
					SourceLevel:     layer.Level,
					SourceScopeID:   layer.ScopeID,
					IsLocked:        true,
					LockReason:      reason,
					LockSourceLevel: layer.Level,
				}
				continue
			}
			if cur.IsLocked {
				// First lock wins; this lock is ignored.
				continue
			}
			cur.IsLocked = true
			cur.LockReason = reason
			cur.LockSourceLevel = layer.Level
			effective[k] = cur
		}
	}

	return effective, nil
}

// ResolveKey resolves a single key. Returns (zero, false, nil) when the
// key is not set in any layer.
func (r *LayeredResolver) ResolveKey(layers []AuthorityLayer, key string) (EffectiveSetting, bool, error) {
	all, err := r.Resolve(layers)
	if err != nil {
		return EffectiveSetting{}, false, err
	}
	v, ok := all[key]
	return v, ok, nil
}

// IsLowerAuthority returns true when a is strictly lower in the hierarchy than b.
func IsLowerAuthority(a, b AuthorityLevel) bool {
	return a < b
}

// IsHigherAuthority returns true when a is strictly higher in the hierarchy than b.
func IsHigherAuthority(a, b AuthorityLevel) bool {
	return a > b
}
