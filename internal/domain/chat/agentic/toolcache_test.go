package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestNewToolResultCache_DefaultCapacity(t *testing.T) {
	c := agentic.NewToolResultCache(0)
	require.NotNil(t, c)
	assert.Equal(t, 0, c.Size())
}

func TestToolResultCache_PutAndGet(t *testing.T) {
	c := agentic.NewToolResultCache(10)

	input := json.RawMessage(`{"query":"test"}`)
	result := agentic.ToolExecResult{
		Output:    json.RawMessage(`{"data":"hello"}`),
		LatencyMs: 42,
	}

	c.Put("document_search", input, result)
	assert.Equal(t, 1, c.Size())

	got := c.Get("document_search", input)
	require.NotNil(t, got)
	assert.Equal(t, result.Output, got.Output)
	assert.Equal(t, int64(42), got.LatencyMs)
}

func TestToolResultCache_Miss(t *testing.T) {
	c := agentic.NewToolResultCache(10)

	got := c.Get("nonexistent", json.RawMessage(`{}`))
	assert.Nil(t, got)

	hits, misses := c.Stats()
	assert.Equal(t, 0, hits)
	assert.Equal(t, 1, misses)
}

func TestToolResultCache_DifferentInputs(t *testing.T) {
	c := agentic.NewToolResultCache(10)

	input1 := json.RawMessage(`{"query":"first"}`)
	input2 := json.RawMessage(`{"query":"second"}`)

	r1 := agentic.ToolExecResult{Output: json.RawMessage(`"result1"`)}
	r2 := agentic.ToolExecResult{Output: json.RawMessage(`"result2"`)}

	c.Put("document_search", input1, r1)
	c.Put("document_search", input2, r2)

	assert.Equal(t, 2, c.Size())

	got1 := c.Get("document_search", input1)
	require.NotNil(t, got1)
	assert.Equal(t, json.RawMessage(`"result1"`), got1.Output)

	got2 := c.Get("document_search", input2)
	require.NotNil(t, got2)
	assert.Equal(t, json.RawMessage(`"result2"`), got2.Output)
}

func TestToolResultCache_DifferentTools(t *testing.T) {
	c := agentic.NewToolResultCache(10)

	input := json.RawMessage(`{"query":"same"}`)

	r1 := agentic.ToolExecResult{Output: json.RawMessage(`"from_search"`)}
	r2 := agentic.ToolExecResult{Output: json.RawMessage(`"from_recall"`)}

	c.Put("document_search", input, r1)
	c.Put("memory_recall", input, r2)

	got1 := c.Get("document_search", input)
	require.NotNil(t, got1)
	assert.Equal(t, json.RawMessage(`"from_search"`), got1.Output)

	got2 := c.Get("memory_recall", input)
	require.NotNil(t, got2)
	assert.Equal(t, json.RawMessage(`"from_recall"`), got2.Output)
}

func TestToolResultCache_LRUEviction(t *testing.T) {
	c := agentic.NewToolResultCache(2)

	r := agentic.ToolExecResult{Output: json.RawMessage(`"x"`)}

	c.Put("tool", json.RawMessage(`"a"`), r)
	c.Put("tool", json.RawMessage(`"b"`), r)
	assert.Equal(t, 2, c.Size())

	// Adding a third entry should evict the oldest ("a").
	c.Put("tool", json.RawMessage(`"c"`), r)
	assert.Equal(t, 2, c.Size())

	assert.Nil(t, c.Get("tool", json.RawMessage(`"a"`)))
	assert.NotNil(t, c.Get("tool", json.RawMessage(`"b"`)))
	assert.NotNil(t, c.Get("tool", json.RawMessage(`"c"`)))
}

func TestToolResultCache_LRURecentAccess(t *testing.T) {
	c := agentic.NewToolResultCache(2)

	r := agentic.ToolExecResult{Output: json.RawMessage(`"x"`)}

	c.Put("tool", json.RawMessage(`"a"`), r)
	c.Put("tool", json.RawMessage(`"b"`), r)

	// Access "a" to make it recently used.
	c.Get("tool", json.RawMessage(`"a"`))

	// Now "b" is the LRU — should be evicted.
	c.Put("tool", json.RawMessage(`"c"`), r)

	assert.NotNil(t, c.Get("tool", json.RawMessage(`"a"`)))
	assert.Nil(t, c.Get("tool", json.RawMessage(`"b"`)))
	assert.NotNil(t, c.Get("tool", json.RawMessage(`"c"`)))
}

func TestToolResultCache_UpdateExisting(t *testing.T) {
	c := agentic.NewToolResultCache(10)

	input := json.RawMessage(`{"q":"test"}`)
	r1 := agentic.ToolExecResult{Output: json.RawMessage(`"old"`)}
	r2 := agentic.ToolExecResult{Output: json.RawMessage(`"new"`)}

	c.Put("tool", input, r1)
	c.Put("tool", input, r2)

	assert.Equal(t, 1, c.Size())
	got := c.Get("tool", input)
	require.NotNil(t, got)
	assert.Equal(t, json.RawMessage(`"new"`), got.Output)
}

func TestToolResultCache_Stats(t *testing.T) {
	c := agentic.NewToolResultCache(10)

	input := json.RawMessage(`{"q":"test"}`)
	r := agentic.ToolExecResult{Output: json.RawMessage(`"ok"`)}

	c.Put("tool", input, r)

	c.Get("tool", input)            // hit
	c.Get("tool", input)            // hit
	c.Get("other", json.RawMessage(`{}`)) // miss

	hits, misses := c.Stats()
	assert.Equal(t, 2, hits)
	assert.Equal(t, 1, misses)
}

func TestIsCacheable(t *testing.T) {
	assert.True(t, agentic.IsCacheable("document_search"))
	assert.True(t, agentic.IsCacheable("memory_recall"))
	assert.False(t, agentic.IsCacheable("execute-sql"))
	assert.False(t, agentic.IsCacheable("http-post"))
	assert.False(t, agentic.IsCacheable(""))
}

func TestDefaultRunConfig_ToolCacheCapacity(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	assert.Equal(t, 64, cfg.ToolCacheCapacity)
}

func TestToolResultCache_ConcurrentAccess(t *testing.T) {
	c := agentic.NewToolResultCache(100)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			input := json.RawMessage(`{"n":` + string(rune('0'+n)) + `}`)
			r := agentic.ToolExecResult{Output: json.RawMessage(`"ok"`)}
			for j := 0; j < 50; j++ {
				c.Put("tool", input, r)
				c.Get("tool", input)
				c.Size()
				c.Stats()
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// No panics = success.
}
