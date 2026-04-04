package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- MemoryType values ---

func TestMemoryType_Values(t *testing.T) {
	assert.Equal(t, agentic.MemoryType("user"), agentic.MemoryTypeUser)
	assert.Equal(t, agentic.MemoryType("feedback"), agentic.MemoryTypeFeedback)
	assert.Equal(t, agentic.MemoryType("project"), agentic.MemoryTypeProject)
	assert.Equal(t, agentic.MemoryType("reference"), agentic.MemoryTypeReference)
}

// --- DefaultMemorySelectorConfig ---

func TestDefaultMemorySelectorConfig(t *testing.T) {
	cfg := agentic.DefaultMemorySelectorConfig()
	assert.Equal(t, 5, cfg.MaxResults)
	assert.Equal(t, 0.1, cfg.MinScore)
	assert.Equal(t, 0.2, cfg.FreshnessWeight)
}

// --- NewMemorySelector ---

func TestNewMemorySelector(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), nil)
	assert.NotNil(t, s)
	assert.Equal(t, 0, s.SurfacedCount())
}

// --- FindRelevant ---

func TestMemorySelector_FindRelevant(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), func(h agentic.MemoryHeader, q string) float64 {
		if h.Filename == "auth.md" {
			return 0.9
		}
		return 0.05 // below threshold
	})

	headers := []agentic.MemoryHeader{
		{Filename: "auth.md", FilePath: "/mem/auth.md", MtimeMs: time.Now().UnixMilli()},
		{Filename: "other.md", FilePath: "/mem/other.md", MtimeMs: time.Now().UnixMilli()},
	}

	results := s.FindRelevant(headers, "authentication")
	assert.Len(t, results, 1)
	assert.Equal(t, "auth.md", results[0].Header.Filename)
	assert.Greater(t, results[0].Score, 0.0)
}

func TestMemorySelector_FindRelevant_MaxResults(t *testing.T) {
	cfg := agentic.DefaultMemorySelectorConfig()
	cfg.MaxResults = 2
	s := agentic.NewMemorySelector(cfg, func(h agentic.MemoryHeader, q string) float64 {
		return 0.5
	})

	headers := make([]agentic.MemoryHeader, 5)
	for i := range headers {
		headers[i] = agentic.MemoryHeader{
			Filename: "mem.md",
			FilePath: "/mem/" + string(rune('a'+i)) + ".md",
			MtimeMs:  time.Now().UnixMilli(),
		}
	}

	results := s.FindRelevant(headers, "test")
	assert.Len(t, results, 2)
}

func TestMemorySelector_FindRelevant_ExcludesSurfaced(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), func(h agentic.MemoryHeader, q string) float64 {
		return 0.8
	})

	headers := []agentic.MemoryHeader{
		{Filename: "a.md", FilePath: "/mem/a.md", MtimeMs: time.Now().UnixMilli()},
		{Filename: "b.md", FilePath: "/mem/b.md", MtimeMs: time.Now().UnixMilli()},
	}

	s.MarkSurfaced("/mem/a.md")
	results := s.FindRelevant(headers, "test")
	assert.Len(t, results, 1)
	assert.Equal(t, "b.md", results[0].Header.Filename)
}

func TestMemorySelector_FindRelevant_SortedByScore(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), func(h agentic.MemoryHeader, q string) float64 {
		switch h.Filename {
		case "high.md":
			return 0.9
		case "mid.md":
			return 0.5
		case "low.md":
			return 0.3
		}
		return 0
	})

	headers := []agentic.MemoryHeader{
		{Filename: "low.md", FilePath: "/mem/low.md", MtimeMs: time.Now().UnixMilli()},
		{Filename: "high.md", FilePath: "/mem/high.md", MtimeMs: time.Now().UnixMilli()},
		{Filename: "mid.md", FilePath: "/mem/mid.md", MtimeMs: time.Now().UnixMilli()},
	}

	results := s.FindRelevant(headers, "test")
	require.Len(t, results, 3)
	assert.Equal(t, "high.md", results[0].Header.Filename)
	assert.Equal(t, "mid.md", results[1].Header.Filename)
	assert.Equal(t, "low.md", results[2].Header.Filename)
}

func TestMemorySelector_FindRelevant_FreshnessBoost(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), func(h agentic.MemoryHeader, q string) float64 {
		return 0.5 // same base score
	})

	now := time.Now()
	headers := []agentic.MemoryHeader{
		{Filename: "old.md", FilePath: "/mem/old.md", MtimeMs: now.Add(-30 * 24 * time.Hour).UnixMilli()},
		{Filename: "new.md", FilePath: "/mem/new.md", MtimeMs: now.UnixMilli()},
	}

	results := s.FindRelevant(headers, "test")
	require.Len(t, results, 2)
	// Newer memory should have higher final score due to freshness
	assert.Equal(t, "new.md", results[0].Header.Filename)
}

// --- MarkSurfaced ---

func TestMemorySelector_MarkSurfaced(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), nil)
	s.MarkSurfaced("/mem/a.md")
	assert.Equal(t, 1, s.SurfacedCount())
}

func TestMemorySelector_MarkSurfacedBatch(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), nil)
	s.MarkSurfacedBatch([]string{"/mem/a.md", "/mem/b.md", "/mem/c.md"})
	assert.Equal(t, 3, s.SurfacedCount())
}

func TestMemorySelector_ClearSurfaced(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), nil)
	s.MarkSurfaced("/mem/a.md")
	s.ClearSurfaced()
	assert.Equal(t, 0, s.SurfacedCount())
}

// --- FormatMemoryManifest ---

func TestFormatMemoryManifest(t *testing.T) {
	headers := []agentic.MemoryHeader{
		{Filename: "auth.md", Type: agentic.MemoryTypeUser, Description: "Auth preferences"},
		{Filename: "project.md", Type: agentic.MemoryTypeProject},
		{Filename: "plain.md"},
	}

	s := agentic.FormatMemoryManifest(headers)
	assert.Contains(t, s, "[auth.md]")
	assert.Contains(t, s, "(user)")
	assert.Contains(t, s, "Auth preferences")
	assert.Contains(t, s, "[project.md]")
	assert.Contains(t, s, "(project)")
	assert.Contains(t, s, "[plain.md]")
}

func TestFormatMemoryManifest_Empty(t *testing.T) {
	s := agentic.FormatMemoryManifest(nil)
	assert.Equal(t, "No memories available.", s)
}

// --- Default relevance score ---

func TestMemorySelector_DefaultScore(t *testing.T) {
	s := agentic.NewMemorySelector(agentic.DefaultMemorySelectorConfig(), nil)

	headers := []agentic.MemoryHeader{
		{Filename: "auth.md", FilePath: "/mem/auth.md", Description: "authentication settings and preferences", MtimeMs: time.Now().UnixMilli()},
		{Filename: "db.md", FilePath: "/mem/db.md", Description: "database configuration", MtimeMs: time.Now().UnixMilli()},
	}

	results := s.FindRelevant(headers, "authentication settings")
	require.GreaterOrEqual(t, len(results), 1)
	assert.Equal(t, "auth.md", results[0].Header.Filename)
}
