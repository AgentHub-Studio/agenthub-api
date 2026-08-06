package agentic

import (
	"os"
	"strings"
	"sync"
	"time"
)

// File content cache with mtime-based invalidation.
//
// Inspired by Claude Code's fileReadCache.ts — caches file contents
// in memory and automatically invalidates when the file's modification
// time changes. Eliminates redundant file reads in tool operations.

// FileReadCache is a thread-safe in-memory cache for file contents
// with automatic invalidation based on modification time.
type FileReadCache struct {
	mu           sync.RWMutex
	cache        map[string]*cachedFileEntry
	maxCacheSize int
}

type cachedFileEntry struct {
	content string
	mtime   time.Time
}

// FileReadCacheStats holds cache statistics for debugging/monitoring.
type FileReadCacheStats struct {
	Size    int
	Entries []string
}

// NewFileReadCache creates a new file read cache with the given maximum
// number of entries. When exceeded, the oldest entry is evicted.
func NewFileReadCache(maxSize int) *FileReadCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &FileReadCache{
		cache:        make(map[string]*cachedFileEntry),
		maxCacheSize: maxSize,
	}
}

// ReadFile reads a file, returning cached content if the file hasn't
// been modified since the last read. Returns the file content and any
// error encountered.
func (c *FileReadCache) ReadFile(path string) (string, error) {
	// Get file stats for invalidation check
	info, err := os.Stat(path)
	if err != nil {
		c.Invalidate(path)
		return "", err
	}

	mtime := info.ModTime()

	// Check cache under read lock
	c.mu.RLock()
	if entry, ok := c.cache[path]; ok && entry.mtime.Equal(mtime) {
		content := entry.content
		c.mu.RUnlock()
		return content, nil
	}
	c.mu.RUnlock()

	// Cache miss — read the file
	data, err := readFileScopedToParent(path)
	if err != nil {
		c.Invalidate(path)
		return "", err
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	// Update cache under write lock
	c.mu.Lock()
	c.cache[path] = &cachedFileEntry{
		content: content,
		mtime:   mtime,
	}

	// Evict oldest if over capacity (simple FIFO via map iteration)
	if len(c.cache) > c.maxCacheSize {
		for k := range c.cache {
			if k != path {
				delete(c.cache, k)
				break
			}
		}
	}
	c.mu.Unlock()

	return content, nil
}

// Invalidate removes a specific file from the cache.
func (c *FileReadCache) Invalidate(path string) {
	c.mu.Lock()
	delete(c.cache, path)
	c.mu.Unlock()
}

// Clear removes all entries from the cache.
func (c *FileReadCache) Clear() {
	c.mu.Lock()
	c.cache = make(map[string]*cachedFileEntry)
	c.mu.Unlock()
}

// Stats returns cache statistics for debugging/monitoring.
func (c *FileReadCache) Stats() FileReadCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]string, 0, len(c.cache))
	for k := range c.cache {
		entries = append(entries, k)
	}
	return FileReadCacheStats{
		Size:    len(c.cache),
		Entries: entries,
	}
}
