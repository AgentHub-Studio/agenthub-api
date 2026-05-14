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

// EXT-007 — Dynamic skill hooks.
//
// PDF arXiv:2604.14228v1 §6.4 (skills can declare their OWN hooks at
// registration time — e.g. a skill says "before any tool call within
// me, run this validator"; the runtime invokes those hooks ONLY when
// that specific skill is active).
//
// Distinct from existing AgentHub plumbing:
//   - hooktype.go = global hook event taxonomy.
//   - EXT-002 hook registry (PARTIAL) = global hook handlers.
//   - EXT-003 HookSchemaRegistry = lifecycle event schemas.
//   - dynamic_skill_hook.go (this file) = SKILL-SCOPED hooks: each
//     skill registers its own handlers at registration time; runtime
//     looks them up keyed by (tenant, skill_slug, phase).

// DynamicSkillHookPhase bounded enum identifies the lifecycle moment.
type DynamicSkillHookPhase string

const (
	// DynamicSkillHookBeforeInvocation — fires before skill body executes.
	DynamicSkillHookBeforeInvocation DynamicSkillHookPhase = "before_invocation"
	// DynamicSkillHookBeforeToolCall — fires before any tool call within skill.
	DynamicSkillHookBeforeToolCall DynamicSkillHookPhase = "before_tool_call"
	// DynamicSkillHookAfterToolCall — fires after a tool call returns.
	DynamicSkillHookAfterToolCall DynamicSkillHookPhase = "after_tool_call"
	// DynamicSkillHookAfterInvocation — fires after skill body completes.
	DynamicSkillHookAfterInvocation DynamicSkillHookPhase = "after_invocation"
	// DynamicSkillHookOnError — fires on skill body error (cleanup).
	DynamicSkillHookOnError DynamicSkillHookPhase = "on_error"
)

var allDynamicSkillHookPhases = []DynamicSkillHookPhase{
	DynamicSkillHookBeforeInvocation, DynamicSkillHookBeforeToolCall,
	DynamicSkillHookAfterToolCall, DynamicSkillHookAfterInvocation,
	DynamicSkillHookOnError,
}

// IsValidDynamicSkillHookPhase returns true for the bounded set.
func IsValidDynamicSkillHookPhase(p DynamicSkillHookPhase) bool {
	for _, v := range allDynamicSkillHookPhases {
		if p == v {
			return true
		}
	}
	return false
}

// AllDynamicSkillHookPhases returns a copy.
func AllDynamicSkillHookPhases() []DynamicSkillHookPhase {
	out := make([]DynamicSkillHookPhase, len(allDynamicSkillHookPhases))
	copy(out, allDynamicSkillHookPhases)
	return out
}

// DynamicSkillHook is one skill-scoped hook registration.
type DynamicSkillHook struct {
	ID            uuid.UUID            `json:"id"`
	TenantID      string               `json:"tenantId"`
	SkillSlug     string               `json:"skillSlug"`
	HookSlug      string               `json:"hookSlug"`
	Phase         DynamicSkillHookPhase `json:"phase"`
	HandlerPath   string               `json:"handlerPath"`
	Priority      int                  `json:"priority"`
	Enabled       bool                 `json:"enabled"`
	RegisteredAt  time.Time            `json:"registeredAt"`
	// SourceExtension identifies the EXT-001 extension that registered
	// this hook (audit + uninstall cleanup).
	SourceExtension string             `json:"sourceExtension,omitempty"`
}

// Sentinels.
var (
	ErrDynSkillHookInvalidPhase     = errors.New("dynamic skill hook: invalid phase")
	ErrDynSkillHookTenantRequired   = errors.New("dynamic skill hook: tenant_id required")
	ErrDynSkillHookSkillSlugReq     = errors.New("dynamic skill hook: skill_slug required")
	ErrDynSkillHookHookSlugReq      = errors.New("dynamic skill hook: hook_slug required")
	ErrDynSkillHookHandlerPathReq   = errors.New("dynamic skill hook: handler_path required")
	ErrDynSkillHookDuplicate        = errors.New("dynamic skill hook: (tenant, skill, hook_slug) already registered")
	ErrDynSkillHookNotFound         = errors.New("dynamic skill hook: not found")
)

// validateDynSkillHook checks structural invariants.
func validateDynSkillHook(h DynamicSkillHook) error {
	if strings.TrimSpace(h.TenantID) == "" {
		return ErrDynSkillHookTenantRequired
	}
	if strings.TrimSpace(h.SkillSlug) == "" {
		return ErrDynSkillHookSkillSlugReq
	}
	if strings.TrimSpace(h.HookSlug) == "" {
		return ErrDynSkillHookHookSlugReq
	}
	if !IsValidDynamicSkillHookPhase(h.Phase) {
		return fmt.Errorf("%w: %q", ErrDynSkillHookInvalidPhase, h.Phase)
	}
	if strings.TrimSpace(h.HandlerPath) == "" {
		return ErrDynSkillHookHandlerPathReq
	}
	return nil
}

// DynamicSkillHookRegistry is the persistence interface.
type DynamicSkillHookRegistry interface {
	Register(ctx context.Context, h DynamicSkillHook) (DynamicSkillHook, error)
	Find(ctx context.Context, tenantID, skillSlug, hookSlug string) (DynamicSkillHook, error)
	HooksFor(ctx context.Context, tenantID, skillSlug string, phase DynamicSkillHookPhase) ([]DynamicSkillHook, error)
	ListBySkill(ctx context.Context, tenantID, skillSlug string) ([]DynamicSkillHook, error)
	ListByExtension(ctx context.Context, tenantID, extensionSlug string) ([]DynamicSkillHook, error)
	Disable(ctx context.Context, tenantID, skillSlug, hookSlug string) error
	Delete(ctx context.Context, tenantID, skillSlug, hookSlug string) error
	ClearExtension(ctx context.Context, tenantID, extensionSlug string) (int, error)
}

// --- InMemoryDynamicSkillHookRegistry ---

type dynHookKey struct {
	tenant string
	skill  string
	hook   string
}

type InMemoryDynamicSkillHookRegistry struct {
	mu    sync.Mutex
	hooks map[dynHookKey]DynamicSkillHook
	now   func() time.Time
}

// NewInMemoryDynamicSkillHookRegistry returns a concurrent-safe registry.
func NewInMemoryDynamicSkillHookRegistry() *InMemoryDynamicSkillHookRegistry {
	return &InMemoryDynamicSkillHookRegistry{
		hooks: map[dynHookKey]DynamicSkillHook{},
		now:   time.Now,
	}
}

// SetClock allows tests to inject a deterministic clock.
func (r *InMemoryDynamicSkillHookRegistry) SetClock(clock func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = clock
}

// Register stores a hook. Rejects duplicates per (tenant, skill, hook_slug).
func (r *InMemoryDynamicSkillHookRegistry) Register(ctx context.Context, h DynamicSkillHook) (DynamicSkillHook, error) {
	if err := ctx.Err(); err != nil {
		return DynamicSkillHook{}, err
	}
	if err := validateDynSkillHook(h); err != nil {
		return DynamicSkillHook{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := dynHookKey{h.TenantID, h.SkillSlug, h.HookSlug}
	if _, exists := r.hooks[key]; exists {
		return DynamicSkillHook{}, fmt.Errorf("%w: %s/%s/%s",
			ErrDynSkillHookDuplicate, h.TenantID, h.SkillSlug, h.HookSlug)
	}
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	if h.RegisteredAt.IsZero() {
		h.RegisteredAt = r.now()
	}
	r.hooks[key] = h
	return h, nil
}

// Find returns one hook by composite key.
func (r *InMemoryDynamicSkillHookRegistry) Find(ctx context.Context, tenantID, skillSlug, hookSlug string) (DynamicSkillHook, error) {
	if err := ctx.Err(); err != nil {
		return DynamicSkillHook{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hooks[dynHookKey{tenantID, skillSlug, hookSlug}]
	if !ok {
		return DynamicSkillHook{}, ErrDynSkillHookNotFound
	}
	return h, nil
}

// HooksFor returns enabled hooks for (tenant, skill, phase) sorted by
// priority desc (higher priority fires first), tie-break by hook_slug.
// Disabled hooks are excluded.
func (r *InMemoryDynamicSkillHookRegistry) HooksFor(ctx context.Context, tenantID, skillSlug string, phase DynamicSkillHookPhase) ([]DynamicSkillHook, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !IsValidDynamicSkillHookPhase(phase) {
		return nil, fmt.Errorf("%w: %q", ErrDynSkillHookInvalidPhase, phase)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []DynamicSkillHook{}
	for k, h := range r.hooks {
		if k.tenant != tenantID || k.skill != skillSlug {
			continue
		}
		if !h.Enabled || h.Phase != phase {
			continue
		}
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].HookSlug < out[j].HookSlug
	})
	return out, nil
}

// ListBySkill returns all hooks (any phase, any enabled state) for a skill.
func (r *InMemoryDynamicSkillHookRegistry) ListBySkill(ctx context.Context, tenantID, skillSlug string) ([]DynamicSkillHook, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []DynamicSkillHook{}
	for k, h := range r.hooks {
		if k.tenant == tenantID && k.skill == skillSlug {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].HookSlug < out[j].HookSlug })
	return out, nil
}

// ListByExtension returns all hooks registered by a given extension
// (used for uninstall cleanup).
func (r *InMemoryDynamicSkillHookRegistry) ListByExtension(ctx context.Context, tenantID, extensionSlug string) ([]DynamicSkillHook, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []DynamicSkillHook{}
	for _, h := range r.hooks {
		if h.TenantID == tenantID && h.SourceExtension == extensionSlug {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SkillSlug != out[j].SkillSlug {
			return out[i].SkillSlug < out[j].SkillSlug
		}
		return out[i].HookSlug < out[j].HookSlug
	})
	return out, nil
}

// Disable marks a hook as disabled (idempotent).
func (r *InMemoryDynamicSkillHookRegistry) Disable(ctx context.Context, tenantID, skillSlug, hookSlug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := dynHookKey{tenantID, skillSlug, hookSlug}
	h, ok := r.hooks[key]
	if !ok {
		return ErrDynSkillHookNotFound
	}
	h.Enabled = false
	r.hooks[key] = h
	return nil
}

// Delete removes a hook.
func (r *InMemoryDynamicSkillHookRegistry) Delete(ctx context.Context, tenantID, skillSlug, hookSlug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := dynHookKey{tenantID, skillSlug, hookSlug}
	if _, ok := r.hooks[key]; !ok {
		return ErrDynSkillHookNotFound
	}
	delete(r.hooks, key)
	return nil
}

// ClearExtension removes all hooks registered by extension. Returns count removed.
func (r *InMemoryDynamicSkillHookRegistry) ClearExtension(ctx context.Context, tenantID, extensionSlug string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for k, h := range r.hooks {
		if h.TenantID == tenantID && h.SourceExtension == extensionSlug {
			delete(r.hooks, k)
			removed++
		}
	}
	return removed, nil
}
