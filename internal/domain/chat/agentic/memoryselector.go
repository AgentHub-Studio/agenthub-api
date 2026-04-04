package agentic

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Intelligent memory selection via semantic relevance ranking.
//
// Inspired by Claude Code's findRelevantMemories.ts — implements a two-stage
// filtering pipeline: header-only scanning (avoids full content load), then
// semantic relevance ranking. Prevents surfacing already-active memories and
// applies a configurable budget to limit context waste.

// MemoryType classifies memory entries.
type MemoryType string

const (
	MemoryTypeUser      MemoryType = "user"
	MemoryTypeFeedback  MemoryType = "feedback"
	MemoryTypeProject   MemoryType = "project"
	MemoryTypeReference MemoryType = "reference"
)

// MemoryHeader holds lightweight metadata for a memory entry.
type MemoryHeader struct {
	Filename    string     `json:"filename"`
	FilePath    string     `json:"filePath"`
	Description string     `json:"description,omitempty"`
	Type        MemoryType `json:"type,omitempty"`
	MtimeMs     int64      `json:"mtimeMs"`
}

// RelevantMemory is a memory selected as relevant to the current context.
type RelevantMemory struct {
	Header   MemoryHeader `json:"header"`
	Score    float64      `json:"score"`
	Selected bool         `json:"selected"`
}

// MemoryRelevanceFunc scores a memory header for relevance to a query.
// Returns a score in [0, 1] where 1 is most relevant.
type MemoryRelevanceFunc func(header MemoryHeader, query string) float64

// MemorySelectorConfig configures the memory selector.
type MemorySelectorConfig struct {
	// MaxResults limits the number of memories returned.
	MaxResults int
	// MinScore is the minimum relevance score to include.
	MinScore float64
	// FreshnessWeight is how much recency influences the final score (0-1).
	FreshnessWeight float64
	// FreshnessHalfLife is the time after which a memory's freshness decays to 50%.
	FreshnessHalfLife time.Duration
}

// DefaultMemorySelectorConfig returns sensible defaults.
func DefaultMemorySelectorConfig() MemorySelectorConfig {
	return MemorySelectorConfig{
		MaxResults:        5,
		MinScore:          0.1,
		FreshnessWeight:   0.2,
		FreshnessHalfLife: 7 * 24 * time.Hour, // 1 week
	}
}

// MemorySelector selects relevant memories for the current context.
type MemorySelector struct {
	mu         sync.Mutex
	config     MemorySelectorConfig
	scoreFn    MemoryRelevanceFunc
	surfaced   map[string]time.Time // filePath → when surfaced (dedup)
}

// NewMemorySelector creates a memory selector.
func NewMemorySelector(config MemorySelectorConfig, scoreFn MemoryRelevanceFunc) *MemorySelector {
	if config.MaxResults <= 0 {
		config.MaxResults = 5
	}
	if scoreFn == nil {
		scoreFn = defaultRelevanceScore
	}
	return &MemorySelector{
		config:   config,
		scoreFn:  scoreFn,
		surfaced: make(map[string]time.Time),
	}
}

// FindRelevant scans headers and returns the most relevant memories for a query.
// Already-surfaced memories are excluded.
func (s *MemorySelector) FindRelevant(headers []MemoryHeader, query string) []RelevantMemory {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var candidates []RelevantMemory

	for _, h := range headers {
		// Skip already-surfaced
		if _, ok := s.surfaced[h.FilePath]; ok {
			continue
		}

		// Score relevance
		baseScore := s.scoreFn(h, query)
		if baseScore < s.config.MinScore {
			continue
		}

		// Apply freshness boost
		freshnessScore := computeFreshness(h.MtimeMs, now, s.config.FreshnessHalfLife)
		finalScore := baseScore*(1-s.config.FreshnessWeight) + freshnessScore*s.config.FreshnessWeight

		candidates = append(candidates, RelevantMemory{
			Header:   h,
			Score:    finalScore,
			Selected: true,
		})
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Trim to budget
	if len(candidates) > s.config.MaxResults {
		candidates = candidates[:s.config.MaxResults]
	}

	return candidates
}

// MarkSurfaced records that a memory was shown to the user/agent.
func (s *MemorySelector) MarkSurfaced(filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.surfaced[filePath] = time.Now()
}

// MarkSurfacedBatch marks multiple memories as surfaced.
func (s *MemorySelector) MarkSurfacedBatch(filePaths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, fp := range filePaths {
		s.surfaced[fp] = now
	}
}

// ClearSurfaced resets the surfaced tracking (e.g., new session).
func (s *MemorySelector) ClearSurfaced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.surfaced = make(map[string]time.Time)
}

// SurfacedCount returns the number of surfaced memories.
func (s *MemorySelector) SurfacedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.surfaced)
}

// FormatManifest formats memory headers into a manifest string
// suitable for inclusion in LLM context.
func FormatMemoryManifest(headers []MemoryHeader) string {
	if len(headers) == 0 {
		return "No memories available."
	}

	var b strings.Builder
	for i, h := range headers {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("- [%s]", h.Filename))
		if h.Type != "" {
			b.WriteString(fmt.Sprintf(" (%s)", h.Type))
		}
		if h.Description != "" {
			b.WriteString(fmt.Sprintf(": %s", h.Description))
		}
	}
	return b.String()
}

// computeFreshness returns a [0,1] score based on recency.
func computeFreshness(mtimeMs int64, now time.Time, halfLife time.Duration) float64 {
	if mtimeMs <= 0 {
		return 0
	}
	mtime := time.UnixMilli(mtimeMs)
	age := now.Sub(mtime)
	if age <= 0 {
		return 1
	}
	if halfLife <= 0 {
		halfLife = 7 * 24 * time.Hour
	}
	// Exponential decay: score = 0.5^(age/halfLife)
	ratio := float64(age) / float64(halfLife)
	score := 1.0
	for i := 0; i < int(ratio); i++ {
		score *= 0.5
	}
	// Fractional part
	frac := ratio - float64(int(ratio))
	if frac > 0 {
		score *= (1.0 - frac*0.5) // linear interpolation within step
	}
	return score
}

// defaultRelevanceScore implements keyword-based scoring.
func defaultRelevanceScore(header MemoryHeader, query string) float64 {
	if query == "" {
		return 0.5 // neutral if no query
	}

	queryLower := strings.ToLower(query)
	score := 0.0

	// Check description match
	if header.Description != "" {
		descLower := strings.ToLower(header.Description)
		words := strings.Fields(queryLower)
		matchCount := 0
		for _, w := range words {
			if len(w) > 2 && strings.Contains(descLower, w) {
				matchCount++
			}
		}
		if len(words) > 0 {
			score = float64(matchCount) / float64(len(words))
		}
	}

	// Check filename match
	filenameLower := strings.ToLower(header.Filename)
	if strings.Contains(queryLower, filenameLower) || strings.Contains(filenameLower, queryLower) {
		score += 0.3
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}
