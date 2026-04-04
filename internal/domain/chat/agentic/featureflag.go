package agentic

import (
	"sync"
	"time"
)

// Dynamic feature flag configuration with async fetch and defaults.
//
// Inspired by Claude Code's useDynamicConfig — returns defaults immediately
// while fetching remote configuration asynchronously. Enables feature flags,
// A/B testing, and runtime parameter tuning without server restart.

// FeatureFlagFetchFunc fetches a config value by name from a remote source.
// Returns the value and whether it was found.
type FeatureFlagFetchFunc func(name string) (interface{}, bool, error)

// FeatureFlagConfig configures the feature flag manager.
type FeatureFlagConfig struct {
	// FetchFunc is called to retrieve config values.
	FetchFunc FeatureFlagFetchFunc
	// CacheTTL is how long fetched values are cached.
	CacheTTL time.Duration
	// FetchTimeout limits individual fetch duration.
	FetchTimeout time.Duration
}

// DefaultFeatureFlagConfig returns sensible defaults.
func DefaultFeatureFlagConfig() FeatureFlagConfig {
	return FeatureFlagConfig{
		CacheTTL:     5 * time.Minute,
		FetchTimeout: 2 * time.Second,
	}
}

type flagEntry struct {
	value     interface{}
	found     bool
	fetchedAt time.Time
	err       error
}

// FeatureFlagManager manages dynamic configuration with caching.
type FeatureFlagManager struct {
	mu       sync.RWMutex
	config   FeatureFlagConfig
	cache    map[string]*flagEntry
	defaults map[string]interface{}
}

// NewFeatureFlagManager creates a feature flag manager.
func NewFeatureFlagManager(config FeatureFlagConfig) *FeatureFlagManager {
	if config.CacheTTL <= 0 {
		config.CacheTTL = 5 * time.Minute
	}
	return &FeatureFlagManager{
		config:   config,
		cache:    make(map[string]*flagEntry),
		defaults: make(map[string]interface{}),
	}
}

// SetDefault registers a default value for a flag.
func (m *FeatureFlagManager) SetDefault(name string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults[name] = value
}

// Get returns the current value of a flag. Returns the cached remote value
// if available and fresh, otherwise returns the default.
func (m *FeatureFlagManager) Get(name string) interface{} {
	m.mu.RLock()
	entry, hasCached := m.cache[name]
	defaultVal := m.defaults[name]
	m.mu.RUnlock()

	if hasCached && entry.found && time.Since(entry.fetchedAt) < m.config.CacheTTL {
		return entry.value
	}

	return defaultVal
}

// GetBool returns a flag as a bool.
func (m *FeatureFlagManager) GetBool(name string) bool {
	v := m.Get(name)
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// GetString returns a flag as a string.
func (m *FeatureFlagManager) GetString(name string) string {
	v := m.Get(name)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetInt returns a flag as an int.
func (m *FeatureFlagManager) GetInt(name string) int {
	v := m.Get(name)
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// GetFloat returns a flag as a float64.
func (m *FeatureFlagManager) GetFloat(name string) float64 {
	v := m.Get(name)
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

// Fetch synchronously fetches a flag value from the remote source
// and updates the cache. Returns the value.
func (m *FeatureFlagManager) Fetch(name string) (interface{}, error) {
	if m.config.FetchFunc == nil {
		return m.Get(name), nil
	}

	val, found, err := m.config.FetchFunc(name)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache[name] = &flagEntry{
		value:     val,
		found:     found,
		fetchedAt: time.Now(),
		err:       err,
	}

	if err != nil {
		// On error, fall back to default
		return m.defaults[name], err
	}

	if !found {
		return m.defaults[name], nil
	}

	return val, nil
}

// Refresh fetches all known flag names (those with defaults).
func (m *FeatureFlagManager) Refresh() map[string]error {
	m.mu.RLock()
	names := make([]string, 0, len(m.defaults))
	for name := range m.defaults {
		names = append(names, name)
	}
	m.mu.RUnlock()

	errs := make(map[string]error)
	for _, name := range names {
		if _, err := m.Fetch(name); err != nil {
			errs[name] = err
		}
	}
	return errs
}

// Invalidate removes a cached value, forcing re-fetch on next Get.
func (m *FeatureFlagManager) Invalidate(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, name)
}

// InvalidateAll clears the entire cache.
func (m *FeatureFlagManager) InvalidateAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]*flagEntry)
}

// CacheSize returns the number of cached entries.
func (m *FeatureFlagManager) CacheSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cache)
}

// IsStale returns true if the cached value for a flag has expired.
func (m *FeatureFlagManager) IsStale(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.cache[name]
	if !ok {
		return true
	}
	return time.Since(entry.fetchedAt) >= m.config.CacheTTL
}
