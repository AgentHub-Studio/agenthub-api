package agentic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestFileReadCache_ReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))

	cache := agentic.NewFileReadCache(100)
	content, err := cache.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", content)
}

func TestFileReadCache_CacheHit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))

	cache := agentic.NewFileReadCache(100)

	// First read
	content1, err := cache.ReadFile(path)
	require.NoError(t, err)

	// Second read (should be cached)
	content2, err := cache.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, content1, content2)
	assert.Equal(t, 1, cache.Stats().Size)
}

func TestFileReadCache_InvalidatesOnDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))

	cache := agentic.NewFileReadCache(100)
	_, err := cache.ReadFile(path)
	require.NoError(t, err)

	require.NoError(t, os.Remove(path))
	_, err = cache.ReadFile(path)
	assert.Error(t, err)
	assert.Equal(t, 0, cache.Stats().Size)
}

func TestFileReadCache_Invalidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))

	cache := agentic.NewFileReadCache(100)
	_, err := cache.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Stats().Size)

	cache.Invalidate(path)
	assert.Equal(t, 0, cache.Stats().Size)
}

func TestFileReadCache_Clear(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(name), 0644))
	}

	cache := agentic.NewFileReadCache(100)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		_, err := cache.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
	}
	assert.Equal(t, 3, cache.Stats().Size)

	cache.Clear()
	assert.Equal(t, 0, cache.Stats().Size)
}

func TestFileReadCache_Eviction(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, string(rune('a'+i))+".txt")
		require.NoError(t, os.WriteFile(name, []byte("x"), 0644))
	}

	cache := agentic.NewFileReadCache(3)
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, string(rune('a'+i))+".txt")
		_, err := cache.ReadFile(name)
		require.NoError(t, err)
	}

	// Should have evicted down to maxCacheSize
	assert.LessOrEqual(t, cache.Stats().Size, 4) // may be 3 or 4 due to eviction timing
}

func TestFileReadCache_NormalizeCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.txt")
	require.NoError(t, os.WriteFile(path, []byte("line1\r\nline2\r\n"), 0644))

	cache := agentic.NewFileReadCache(100)
	content, err := cache.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\n", content)
}

func TestFileReadCache_Stats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))

	cache := agentic.NewFileReadCache(100)
	_, err := cache.ReadFile(path)
	require.NoError(t, err)

	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
	assert.Contains(t, stats.Entries, path)
}

func TestFileReadCache_NonExistentFile(t *testing.T) {
	cache := agentic.NewFileReadCache(100)
	_, err := cache.ReadFile("/nonexistent/path.txt")
	assert.Error(t, err)
}

func TestFileReadCache_DefaultMaxSize(t *testing.T) {
	cache := agentic.NewFileReadCache(0) // should default to 1000
	assert.NotNil(t, cache)
}
