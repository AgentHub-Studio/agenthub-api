package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

// ToolResultCache is an LRU cache for tool execution results.
// It is scoped to a single agentic run and caches results of read-only
// idempotent tools to avoid redundant re-execution within the same run.
type ToolResultCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*cacheEntry
	order    []string // LRU order: newest at end
	hits     int
	misses   int
}

type cacheEntry struct {
	key    string
	result ToolExecResult
}

// NewToolResultCache creates a cache with the given capacity.
// If capacity <= 0, defaults to 64.
func NewToolResultCache(capacity int) *ToolResultCache {
	if capacity <= 0 {
		capacity = 64
	}
	return &ToolResultCache{
		capacity: capacity,
		entries:  make(map[string]*cacheEntry, capacity),
		order:    make([]string, 0, capacity),
	}
}

// cacheKey builds a deterministic key from tool name and input.
func cacheKey(toolName string, input json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte{0}) // separator
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))[:16] // 16 hex chars is plenty for a run-scoped cache
}

// Get retrieves a cached result. Returns nil if not cached.
func (c *ToolResultCache) Get(toolName string, input json.RawMessage) *ToolExecResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(toolName, input)
	entry, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil
	}

	// Move to end of LRU order.
	c.moveToEnd(key)
	c.hits++

	result := entry.result
	return &result
}

// Put stores a tool result in the cache. Evicts the least recently used
// entry if the cache is at capacity.
func (c *ToolResultCache) Put(toolName string, input json.RawMessage, result ToolExecResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(toolName, input)

	// Update existing entry.
	if _, ok := c.entries[key]; ok {
		c.entries[key].result = result
		c.moveToEnd(key)
		return
	}

	// Evict if at capacity.
	if len(c.entries) >= c.capacity {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{key: key, result: result}
	c.order = append(c.order, key)
}

// Stats returns hit/miss statistics.
func (c *ToolResultCache) Stats() (hits, misses int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses
}

// Size returns the number of entries in the cache.
func (c *ToolResultCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *ToolResultCache) moveToEnd(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}

func (c *ToolResultCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, oldest)
}

// CacheableTools defines which tools are safe to cache (read-only, idempotent).
// This is the default set; it can be extended via RunConfig.
var CacheableTools = map[string]bool{
	"document_search": true,
	"memory_recall":   true,
}

// IsCacheable returns true if a tool's results can be cached.
func IsCacheable(toolName string) bool {
	return CacheableTools[toolName]
}
