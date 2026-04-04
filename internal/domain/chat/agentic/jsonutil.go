package agentic

import (
	"encoding/json"
	"strings"
	"sync"
	"unicode/utf8"
)

// Safe JSON parsing with LRU caching and JSONL support.
//
// Inspired by Claude Code's json.ts — LRU-bounded JSON parsing that
// caches small inputs, BOM stripping, and line-by-line JSONL parsing
// that skips malformed lines. Useful for repeatedly parsing the same
// config strings and for streaming log/event ingestion.

const (
	// parseCacheMaxKeyBytes is the max input size eligible for caching.
	// Large inputs change between reads and pin too much memory.
	parseCacheMaxKeyBytes = 8 * 1024

	// defaultJSONParseCacheSize is the LRU capacity.
	defaultJSONParseCacheSize = 50
)

type jsonCacheEntry struct {
	value interface{}
	ok    bool
}

// JSONParseCache is an LRU cache for parsed JSON values.
type JSONParseCache struct {
	mu       sync.Mutex
	capacity int
	keys     []string
	entries  map[string]jsonCacheEntry
}

// NewJSONParseCache creates a JSON parse cache with the given capacity.
func NewJSONParseCache(capacity int) *JSONParseCache {
	if capacity <= 0 {
		capacity = defaultJSONParseCacheSize
	}
	return &JSONParseCache{
		capacity: capacity,
		entries:  make(map[string]jsonCacheEntry, capacity),
	}
}

// Parse parses a JSON string, returning the result and whether it was
// valid. Results for small inputs are cached; repeated calls with the
// same string return the cached value.
func (c *JSONParseCache) Parse(input string) (interface{}, bool) {
	if len(input) > parseCacheMaxKeyBytes {
		return parseJSONOnce(input)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, found := c.entries[input]; found {
		return entry.value, entry.ok
	}

	val, ok := parseJSONOnce(input)
	entry := jsonCacheEntry{value: val, ok: ok}

	if len(c.keys) >= c.capacity {
		evict := c.keys[0]
		c.keys = c.keys[1:]
		delete(c.entries, evict)
	}
	c.keys = append(c.keys, input)
	c.entries[input] = entry

	return val, ok
}

// Size returns the number of cached entries.
func (c *JSONParseCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func parseJSONOnce(input string) (interface{}, bool) {
	cleaned := StripBOM(input)
	var result interface{}
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, false
	}
	return result, true
}

// SafeParseJSON parses a JSON string, returning nil on error.
// Small inputs (<=8KB) are cached in a global LRU.
func SafeParseJSON(input string) interface{} {
	if input == "" {
		return nil
	}
	val, ok := globalJSONCache.Parse(input)
	if !ok {
		return nil
	}
	return val
}

var globalJSONCache = NewJSONParseCache(defaultJSONParseCacheSize)

// StripBOM removes a UTF-8 BOM (byte order mark) from the start of a
// string if present. PowerShell 5.x adds BOM to UTF-8 files.
func StripBOM(s string) string {
	if len(s) >= 3 && s[0] == 0xEF && s[1] == 0xBB && s[2] == 0xBF {
		return s[3:]
	}
	// Also handle the Unicode BOM character directly.
	if r, size := utf8.DecodeRuneInString(s); r == '\uFEFF' {
		return s[size:]
	}
	return s
}

// ParseJSONL parses newline-delimited JSON (JSONL), skipping blank
// and malformed lines. Returns a slice of parsed values.
func ParseJSONL(data string) []interface{} {
	stripped := StripBOM(data)
	if stripped == "" {
		return nil
	}

	var results []interface{}
	for {
		idx := strings.IndexByte(stripped, '\n')
		var line string
		if idx == -1 {
			line = strings.TrimSpace(stripped)
			if line != "" {
				var val interface{}
				if json.Unmarshal([]byte(line), &val) == nil {
					results = append(results, val)
				}
			}
			break
		}
		line = strings.TrimSpace(stripped[:idx])
		stripped = stripped[idx+1:]
		if line == "" {
			continue
		}
		var val interface{}
		if json.Unmarshal([]byte(line), &val) == nil {
			results = append(results, val)
		}
	}
	return results
}

// ParseJSONLTyped parses JSONL into a slice of a specific type,
// skipping lines that fail to unmarshal.
func ParseJSONLTyped[T any](data string) []T {
	stripped := StripBOM(data)
	if stripped == "" {
		return nil
	}

	var results []T
	for {
		idx := strings.IndexByte(stripped, '\n')
		var line string
		if idx == -1 {
			line = strings.TrimSpace(stripped)
			if line != "" {
				var val T
				if json.Unmarshal([]byte(line), &val) == nil {
					results = append(results, val)
				}
			}
			break
		}
		line = strings.TrimSpace(stripped[:idx])
		stripped = stripped[idx+1:]
		if line == "" {
			continue
		}
		var val T
		if json.Unmarshal([]byte(line), &val) == nil {
			results = append(results, val)
		}
	}
	return results
}
