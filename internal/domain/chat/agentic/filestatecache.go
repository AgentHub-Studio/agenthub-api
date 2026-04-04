package agentic

import (
	"path/filepath"
	"sync"
)

// LRU file state cache with size-based eviction.
//
// Inspired by Claude Code's fileStateCache.ts — caches file content
// with path normalization, partial view tracking, and configurable
// limits on both entry count and total byte size.

// DefaultMaxCacheEntries is the default maximum number of cached files.
const DefaultMaxCacheEntries = 100

// DefaultMaxCacheSizeBytes is the default maximum total cache size (25 MB).
const DefaultMaxCacheSizeBytes = 25 * 1024 * 1024

// FileState represents the cached state of a single file.
type FileState struct {
	// Content holds the file content (or raw disk bytes for partial views).
	Content string `json:"content"`
	// Timestamp is the time this entry was last updated (Unix nanos).
	Timestamp int64 `json:"timestamp"`
	// Offset is the line offset of a partial read (nil = full read).
	Offset *int `json:"offset,omitempty"`
	// Limit is the line limit of a partial read (nil = full read).
	Limit *int `json:"limit,omitempty"`
	// IsPartialView indicates the model saw content different from disk
	// (e.g., auto-injected, HTML-stripped, truncated).
	IsPartialView bool `json:"isPartialView,omitempty"`
}

// fileStateCacheEntry is an internal doubly-linked list node.
type fileStateCacheEntry struct {
	key   string
	value FileState
	size  int
	prev  *fileStateCacheEntry
	next  *fileStateCacheEntry
}

// FileStateCache is a thread-safe LRU cache for file states.
type FileStateCache struct {
	mu           sync.Mutex
	maxEntries   int
	maxSizeBytes int
	currentSize  int

	entries map[string]*fileStateCacheEntry
	head    *fileStateCacheEntry // most recently used
	tail    *fileStateCacheEntry // least recently used
}

// NewFileStateCache creates a file state cache with the given limits.
func NewFileStateCache(maxEntries, maxSizeBytes int) *FileStateCache {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxCacheEntries
	}
	if maxSizeBytes <= 0 {
		maxSizeBytes = DefaultMaxCacheSizeBytes
	}
	return &FileStateCache{
		maxEntries:   maxEntries,
		maxSizeBytes: maxSizeBytes,
		entries:      make(map[string]*fileStateCacheEntry),
	}
}

// normKey normalizes a file path for consistent cache hits.
func normKey(key string) string {
	return filepath.Clean(key)
}

// Get retrieves a cached file state. Returns the state and true if found.
func (c *FileStateCache) Get(key string) (FileState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[normKey(key)]
	if !ok {
		return FileState{}, false
	}

	c.moveToFront(e)
	return e.value, true
}

// Set stores a file state in the cache.
func (c *FileStateCache) Set(key string, value FileState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	nk := normKey(key)
	entrySize := len(value.Content)
	if entrySize < 1 {
		entrySize = 1
	}

	if e, ok := c.entries[nk]; ok {
		c.currentSize -= e.size
		e.value = value
		e.size = entrySize
		c.currentSize += entrySize
		c.moveToFront(e)
	} else {
		e := &fileStateCacheEntry{key: nk, value: value, size: entrySize}
		c.entries[nk] = e
		c.pushFront(e)
		c.currentSize += entrySize
	}

	c.evict()
}

// Has returns true if the key exists in the cache.
func (c *FileStateCache) Has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.entries[normKey(key)]
	return ok
}

// Delete removes a key from the cache. Returns true if found.
func (c *FileStateCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	nk := normKey(key)
	e, ok := c.entries[nk]
	if !ok {
		return false
	}

	c.removeEntry(e)
	return true
}

// Clear removes all entries.
func (c *FileStateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*fileStateCacheEntry)
	c.head = nil
	c.tail = nil
	c.currentSize = 0
}

// Keys returns all cached keys in MRU order.
func (c *FileStateCache) Keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]string, 0, len(c.entries))
	for e := c.head; e != nil; e = e.next {
		keys = append(keys, e.key)
	}
	return keys
}

// Size returns the number of cached entries.
func (c *FileStateCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// CurrentSizeBytes returns the total bytes of cached content.
func (c *FileStateCache) CurrentSizeBytes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentSize
}

// MaxEntries returns the configured entry limit.
func (c *FileStateCache) MaxEntries() int {
	return c.maxEntries
}

// MaxSizeBytes returns the configured byte limit.
func (c *FileStateCache) MaxSizeBytes() int {
	return c.maxSizeBytes
}

// Clone creates a deep copy of the cache with the same configuration.
func (c *FileStateCache) Clone() *FileStateCache {
	c.mu.Lock()
	defer c.mu.Unlock()

	clone := NewFileStateCache(c.maxEntries, c.maxSizeBytes)

	// Walk from tail to head so the most recent ends up at front.
	for e := c.tail; e != nil; e = e.prev {
		clone.entries[e.key] = &fileStateCacheEntry{key: e.key, value: e.value, size: e.size}
		clone.pushFront(clone.entries[e.key])
		clone.currentSize += e.size
	}

	return clone
}

// Merge combines another cache into this one. For duplicate keys,
// the entry with the newer timestamp wins.
func (c *FileStateCache) Merge(other *FileStateCache) {
	other.mu.Lock()
	entries := make([]struct {
		key   string
		value FileState
	}, 0, len(other.entries))
	for e := other.tail; e != nil; e = e.prev {
		entries = append(entries, struct {
			key   string
			value FileState
		}{e.key, e.value})
	}
	other.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range entries {
		existing, ok := c.entries[entry.key]
		if ok && existing.value.Timestamp >= entry.value.Timestamp {
			continue // existing is newer
		}

		entrySize := len(entry.value.Content)
		if entrySize < 1 {
			entrySize = 1
		}

		if ok {
			c.currentSize -= existing.size
			existing.value = entry.value
			existing.size = entrySize
			c.currentSize += entrySize
			c.moveToFront(existing)
		} else {
			e := &fileStateCacheEntry{key: entry.key, value: entry.value, size: entrySize}
			c.entries[entry.key] = e
			c.pushFront(e)
			c.currentSize += entrySize
		}
	}

	c.evict()
}

// --- Internal linked-list operations ---

func (c *FileStateCache) pushFront(e *fileStateCacheEntry) {
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

func (c *FileStateCache) moveToFront(e *fileStateCacheEntry) {
	if c.head == e {
		return
	}
	c.unlink(e)
	c.pushFront(e)
}

func (c *FileStateCache) unlink(e *fileStateCacheEntry) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}

func (c *FileStateCache) removeEntry(e *fileStateCacheEntry) {
	c.unlink(e)
	delete(c.entries, e.key)
	c.currentSize -= e.size
}

func (c *FileStateCache) evict() {
	for len(c.entries) > c.maxEntries && c.tail != nil {
		c.removeEntry(c.tail)
	}
	for c.currentSize > c.maxSizeBytes && c.tail != nil {
		c.removeEntry(c.tail)
	}
}
