package agentic

import (
	"sync"
	"time"
)

// Enterprise policy limits verification.
//
// Inspired by Claude Code's policyLimits — enforces enterprise-level
// restrictions on tool usage and features. Uses ETag-based caching
// with absence-as-allowed semantics: only blocked policies are listed.

// PolicyRestriction represents a single policy restriction.
type PolicyRestriction struct {
	// Allowed indicates whether the action is permitted.
	Allowed bool `json:"allowed"`
}

// PolicyLimits holds all current policy restrictions.
type PolicyLimits struct {
	// Restrictions maps policy names to their restriction state.
	// Absence of a key means the action is allowed.
	Restrictions map[string]PolicyRestriction `json:"restrictions"`
	// ETag is the server-provided cache tag.
	ETag string `json:"etag,omitempty"`
	// FetchedAt is when the limits were last fetched.
	FetchedAt time.Time `json:"fetchedAt"`
}

// PolicyFetchResult represents the result of a policy limits fetch.
type PolicyFetchResult struct {
	// Limits holds the fetched policy data (nil for 304 Not Modified).
	Limits *PolicyLimits `json:"limits,omitempty"`
	// NotModified is true if the server returned 304.
	NotModified bool `json:"notModified,omitempty"`
	// SkipRetry is true for permanent auth errors (4xx except 409/429).
	SkipRetry bool `json:"skipRetry,omitempty"`
	// Error is any fetch error.
	Error error `json:"error,omitempty"`
}

// PolicyLimitsCache caches and serves policy limits with ETag support.
type PolicyLimitsCache struct {
	mu     sync.RWMutex
	limits *PolicyLimits
	ttl    time.Duration
}

// DefaultPolicyTTL is the default cache TTL (1 hour).
const DefaultPolicyTTL = time.Hour

// NewPolicyLimitsCache creates a policy limits cache.
func NewPolicyLimitsCache(ttl time.Duration) *PolicyLimitsCache {
	if ttl <= 0 {
		ttl = DefaultPolicyTTL
	}
	return &PolicyLimitsCache{ttl: ttl}
}

// Update stores new policy limits.
func (c *PolicyLimitsCache) Update(limits PolicyLimits) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if limits.FetchedAt.IsZero() {
		limits.FetchedAt = time.Now()
	}
	c.limits = &limits
}

// IsAllowed checks if a specific action is allowed.
// Returns true if the policy is not restricted or not present (absence = allowed).
func (c *PolicyLimitsCache) IsAllowed(policyName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.limits == nil {
		return true
	}

	restriction, exists := c.limits.Restrictions[policyName]
	if !exists {
		return true // absence = allowed
	}
	return restriction.Allowed
}

// GetBlockedPolicies returns all currently blocked policy names.
func (c *PolicyLimitsCache) GetBlockedPolicies() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.limits == nil {
		return nil
	}

	var blocked []string
	for name, r := range c.limits.Restrictions {
		if !r.Allowed {
			blocked = append(blocked, name)
		}
	}
	return blocked
}

// IsStale returns true if the cached limits are older than TTL.
func (c *PolicyLimitsCache) IsStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.limits == nil {
		return true
	}
	return time.Since(c.limits.FetchedAt) > c.ttl
}

// ETag returns the current ETag for conditional fetching.
func (c *PolicyLimitsCache) ETag() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.limits == nil {
		return ""
	}
	return c.limits.ETag
}

// HasLimits returns whether any limits have been loaded.
func (c *PolicyLimitsCache) HasLimits() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.limits != nil
}

// RestrictionCount returns the number of active restrictions.
func (c *PolicyLimitsCache) RestrictionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.limits == nil {
		return 0
	}
	return len(c.limits.Restrictions)
}

// Clear removes all cached limits.
func (c *PolicyLimitsCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limits = nil
}
