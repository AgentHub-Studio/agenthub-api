package agentic

import (
	"context"
	"sync"
)

// In-process concurrent agent context isolation.
//
// Inspired by Claude Code's teammateContext.ts — enables multiple
// agents to run concurrently in the same process without global state
// conflicts. Uses Go's context.Context for goroutine-scoped isolation
// (analogous to Node.js AsyncLocalStorage).

// TeammateContext holds identity and configuration for an in-process agent.
type TeammateContext struct {
	// AgentID uniquely identifies this teammate.
	AgentID string `json:"agentId"`
	// AgentName is the human-readable name.
	AgentName string `json:"agentName"`
	// TeamName is the team this agent belongs to.
	TeamName string `json:"teamName"`
	// Color is the agent's display color.
	Color string `json:"color,omitempty"`
	// ParentSessionID links to the parent session.
	ParentSessionID string `json:"parentSessionId,omitempty"`
	// IsInProcess distinguishes in-process from env-var teammates.
	IsInProcess bool `json:"isInProcess"`
	// PlanModeRequired indicates if the agent must use plan mode.
	PlanModeRequired bool `json:"planModeRequired,omitempty"`
}

// teammateContextKey is the context key for TeammateContext.
type teammateContextKey struct{}

// WithTeammateContext returns a new context with the teammate context set.
func WithTeammateContext(ctx context.Context, tc *TeammateContext) context.Context {
	return context.WithValue(ctx, teammateContextKey{}, tc)
}

// GetTeammateContext retrieves the teammate context from a Go context.
// Returns nil if not set.
func GetTeammateContext(ctx context.Context) *TeammateContext {
	tc, _ := ctx.Value(teammateContextKey{}).(*TeammateContext)
	return tc
}

// IsInProcessTeammate returns true if the context belongs to an in-process teammate.
func IsInProcessTeammate(ctx context.Context) bool {
	tc := GetTeammateContext(ctx)
	return tc != nil && tc.IsInProcess
}

// RunWithTeammateContext executes a function with a teammate context.
func RunWithTeammateContext[T any](ctx context.Context, tc *TeammateContext, fn func(ctx context.Context) (T, error)) (T, error) {
	return fn(WithTeammateContext(ctx, tc))
}

// TeammateRegistry tracks active in-process teammates.
type TeammateRegistry struct {
	mu        sync.RWMutex
	teammates map[string]*TeammateContext
}

// NewTeammateRegistry creates a teammate registry.
func NewTeammateRegistry() *TeammateRegistry {
	return &TeammateRegistry{
		teammates: make(map[string]*TeammateContext),
	}
}

// Register adds a teammate to the registry.
func (r *TeammateRegistry) Register(tc *TeammateContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.teammates[tc.AgentID] = tc
}

// Unregister removes a teammate from the registry.
func (r *TeammateRegistry) Unregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.teammates, agentID)
}

// Get returns a teammate by ID.
func (r *TeammateRegistry) Get(agentID string) *TeammateContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tc, ok := r.teammates[agentID]
	if !ok {
		return nil
	}
	copy := *tc
	return &copy
}

// GetByTeam returns all teammates in a team.
func (r *TeammateRegistry) GetByTeam(teamName string) []*TeammateContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*TeammateContext
	for _, tc := range r.teammates {
		if tc.TeamName == teamName {
			copy := *tc
			result = append(result, &copy)
		}
	}
	return result
}

// Count returns the number of registered teammates.
func (r *TeammateRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.teammates)
}

// All returns all registered teammates.
func (r *TeammateRegistry) All() []*TeammateContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*TeammateContext, 0, len(r.teammates))
	for _, tc := range r.teammates {
		copy := *tc
		result = append(result, &copy)
	}
	return result
}

// Clear removes all teammates.
func (r *TeammateRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.teammates = make(map[string]*TeammateContext)
}
