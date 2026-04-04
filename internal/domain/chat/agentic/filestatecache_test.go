package agentic_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestFileStateCache_Constants(t *testing.T) {
	assert.Equal(t, 100, agentic.DefaultMaxCacheEntries)
	assert.Equal(t, 25*1024*1024, agentic.DefaultMaxCacheSizeBytes)
}

// --- NewFileStateCache ---

func TestNewFileStateCache_Defaults(t *testing.T) {
	c := agentic.NewFileStateCache(0, 0)
	assert.Equal(t, agentic.DefaultMaxCacheEntries, c.MaxEntries())
	assert.Equal(t, agentic.DefaultMaxCacheSizeBytes, c.MaxSizeBytes())
	assert.Equal(t, 0, c.Size())
}

// --- Get / Set ---

func TestFileStateCache_GetSet(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/tmp/a.go", agentic.FileState{Content: "hello", Timestamp: 100})

	state, ok := c.Get("/tmp/a.go")
	require.True(t, ok)
	assert.Equal(t, "hello", state.Content)
	assert.Equal(t, int64(100), state.Timestamp)
}

func TestFileStateCache_Get_NotFound(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	_, ok := c.Get("/no/such/file")
	assert.False(t, ok)
}

func TestFileStateCache_Set_Overwrite(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/tmp/a.go", agentic.FileState{Content: "v1", Timestamp: 1})
	c.Set("/tmp/a.go", agentic.FileState{Content: "v2", Timestamp: 2})

	state, ok := c.Get("/tmp/a.go")
	require.True(t, ok)
	assert.Equal(t, "v2", state.Content)
	assert.Equal(t, 1, c.Size(), "should not double-count")
}

// --- Path normalization ---

func TestFileStateCache_PathNormalization(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/tmp/dir/../a.go", agentic.FileState{Content: "normalized"})

	state, ok := c.Get("/tmp/a.go")
	require.True(t, ok)
	assert.Equal(t, "normalized", state.Content)
}

func TestFileStateCache_PathNormalization_Has(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/tmp/./a.go", agentic.FileState{Content: "x"})
	assert.True(t, c.Has("/tmp/a.go"))
}

func TestFileStateCache_PathNormalization_Delete(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/tmp/a.go", agentic.FileState{Content: "x"})
	assert.True(t, c.Delete("/tmp/./a.go"))
	assert.Equal(t, 0, c.Size())
}

// --- Partial view tracking ---

func TestFileStateCache_PartialView(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	offset := 10
	limit := 20
	c.Set("/tmp/big.go", agentic.FileState{
		Content:       "partial content",
		Timestamp:     1,
		Offset:        &offset,
		Limit:         &limit,
		IsPartialView: true,
	})

	state, ok := c.Get("/tmp/big.go")
	require.True(t, ok)
	assert.True(t, state.IsPartialView)
	assert.Equal(t, 10, *state.Offset)
	assert.Equal(t, 20, *state.Limit)
}

// --- LRU eviction by entry count ---

func TestFileStateCache_EvictByEntryCount(t *testing.T) {
	c := agentic.NewFileStateCache(3, 100*1024*1024)

	c.Set("/a", agentic.FileState{Content: "a"})
	c.Set("/b", agentic.FileState{Content: "b"})
	c.Set("/c", agentic.FileState{Content: "c"})
	c.Set("/d", agentic.FileState{Content: "d"}) // should evict /a (LRU)

	assert.Equal(t, 3, c.Size())
	assert.False(t, c.Has("/a"))
	assert.True(t, c.Has("/b"))
	assert.True(t, c.Has("/d"))
}

func TestFileStateCache_EvictByEntryCount_AccessRenews(t *testing.T) {
	c := agentic.NewFileStateCache(3, 100*1024*1024)

	c.Set("/a", agentic.FileState{Content: "a"})
	c.Set("/b", agentic.FileState{Content: "b"})
	c.Set("/c", agentic.FileState{Content: "c"})

	// Access /a to make it MRU.
	c.Get("/a")

	c.Set("/d", agentic.FileState{Content: "d"}) // should evict /b (now LRU)

	assert.True(t, c.Has("/a"), "/a was accessed, should survive")
	assert.False(t, c.Has("/b"), "/b should be evicted")
}

// --- LRU eviction by size ---

func TestFileStateCache_EvictBySize(t *testing.T) {
	c := agentic.NewFileStateCache(100, 10) // 10 bytes max

	c.Set("/a", agentic.FileState{Content: "aaaa"}) // 4 bytes
	c.Set("/b", agentic.FileState{Content: "bbbb"}) // 4 bytes
	c.Set("/c", agentic.FileState{Content: "cccc"}) // 4 bytes → total 12, evict /a

	assert.False(t, c.Has("/a"))
	assert.True(t, c.Has("/b"))
	assert.True(t, c.Has("/c"))
	assert.LessOrEqual(t, c.CurrentSizeBytes(), 10)
}

// --- Delete ---

func TestFileStateCache_Delete(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/a", agentic.FileState{Content: "x"})

	ok := c.Delete("/a")
	assert.True(t, ok)
	assert.False(t, c.Has("/a"))
	assert.Equal(t, 0, c.Size())
}

func TestFileStateCache_Delete_NotFound(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	assert.False(t, c.Delete("/no"))
}

// --- Clear ---

func TestFileStateCache_Clear(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/a", agentic.FileState{Content: "x"})
	c.Set("/b", agentic.FileState{Content: "y"})
	c.Clear()
	assert.Equal(t, 0, c.Size())
	assert.Equal(t, 0, c.CurrentSizeBytes())
}

// --- Keys ---

func TestFileStateCache_Keys_MRUOrder(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/a", agentic.FileState{Content: "a"})
	c.Set("/b", agentic.FileState{Content: "b"})
	c.Set("/c", agentic.FileState{Content: "c"})

	keys := c.Keys()
	require.Len(t, keys, 3)
	assert.Equal(t, "/c", keys[0], "most recently set should be first")
}

// --- Clone ---

func TestFileStateCache_Clone(t *testing.T) {
	c := agentic.NewFileStateCache(10, 1024*1024)
	c.Set("/a", agentic.FileState{Content: "hello", Timestamp: 1})

	cloned := c.Clone()

	state, ok := cloned.Get("/a")
	require.True(t, ok)
	assert.Equal(t, "hello", state.Content)

	// Mutating clone should not affect original.
	cloned.Set("/a", agentic.FileState{Content: "modified"})
	original, _ := c.Get("/a")
	assert.Equal(t, "hello", original.Content)
}

// --- Merge ---

func TestFileStateCache_Merge_NewEntries(t *testing.T) {
	c1 := agentic.NewFileStateCache(10, 1024*1024)
	c1.Set("/a", agentic.FileState{Content: "a1", Timestamp: 1})

	c2 := agentic.NewFileStateCache(10, 1024*1024)
	c2.Set("/b", agentic.FileState{Content: "b2", Timestamp: 2})

	c1.Merge(c2)

	assert.Equal(t, 2, c1.Size())
	state, ok := c1.Get("/b")
	require.True(t, ok)
	assert.Equal(t, "b2", state.Content)
}

func TestFileStateCache_Merge_NewerWins(t *testing.T) {
	c1 := agentic.NewFileStateCache(10, 1024*1024)
	c1.Set("/a", agentic.FileState{Content: "old", Timestamp: 1})

	c2 := agentic.NewFileStateCache(10, 1024*1024)
	c2.Set("/a", agentic.FileState{Content: "new", Timestamp: 5})

	c1.Merge(c2)

	state, _ := c1.Get("/a")
	assert.Equal(t, "new", state.Content)
}

func TestFileStateCache_Merge_OlderIgnored(t *testing.T) {
	c1 := agentic.NewFileStateCache(10, 1024*1024)
	c1.Set("/a", agentic.FileState{Content: "newer", Timestamp: 10})

	c2 := agentic.NewFileStateCache(10, 1024*1024)
	c2.Set("/a", agentic.FileState{Content: "older", Timestamp: 1})

	c1.Merge(c2)

	state, _ := c1.Get("/a")
	assert.Equal(t, "newer", state.Content)
}

// --- Size tracking ---

func TestFileStateCache_SizeTracking(t *testing.T) {
	c := agentic.NewFileStateCache(100, 1024*1024)
	c.Set("/a", agentic.FileState{Content: strings.Repeat("x", 100)})
	c.Set("/b", agentic.FileState{Content: strings.Repeat("y", 200)})

	assert.Equal(t, 300, c.CurrentSizeBytes())

	c.Delete("/a")
	assert.Equal(t, 200, c.CurrentSizeBytes())
}

func TestFileStateCache_SizeTracking_EmptyContent(t *testing.T) {
	c := agentic.NewFileStateCache(100, 1024*1024)
	c.Set("/empty", agentic.FileState{Content: ""})
	assert.Equal(t, 1, c.CurrentSizeBytes(), "empty content counts as 1 byte minimum")
}

// --- Stress test ---

func TestFileStateCache_ManyEntries(t *testing.T) {
	c := agentic.NewFileStateCache(50, 1024*1024)

	for i := 0; i < 200; i++ {
		c.Set(fmt.Sprintf("/file%d.go", i), agentic.FileState{
			Content:   fmt.Sprintf("content-%d", i),
			Timestamp: int64(i),
		})
	}

	assert.Equal(t, 50, c.Size(), "should not exceed max entries")

	// Latest entries should survive.
	_, ok := c.Get("/file199.go")
	assert.True(t, ok)
}
