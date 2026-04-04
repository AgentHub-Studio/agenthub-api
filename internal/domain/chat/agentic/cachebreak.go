package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Prompt cache break diagnosis via snapshot diffing.
//
// Inspired by Claude Code's promptCacheBreakDetection.ts — complements the
// existing CacheBreakDetector (analytics.go) by providing root-cause diagnosis
// via per-tool schema hashing and prompt state snapshot comparison.
// The existing detector answers "did cache break?"; this answers "why?".

// PromptCacheBreakThreshold is the minimum drop in cache read tokens
// (as a fraction of previous baseline) to trigger a break detection.
const PromptCacheBreakThreshold = 0.05

// PromptStateSnapshot captures the state of a prompt configuration
// at a point in time. Used to detect what changed between calls.
type PromptStateSnapshot struct {
	SystemPromptHash string            `json:"systemPromptHash"`
	ToolSchemaHashes map[string]string `json:"toolSchemaHashes"`
	Model            string            `json:"model"`
	AgentID          string            `json:"agentId,omitempty"`
	FastMode         bool              `json:"fastMode,omitempty"`
	CacheStrategy    string            `json:"cacheStrategy,omitempty"`
	EffortValue      string            `json:"effortValue,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
}

// PromptCacheBreakChange represents a detected change between two snapshots.
type PromptCacheBreakChange struct {
	SystemPromptChanged  bool     `json:"systemPromptChanged"`
	ModelChanged         bool     `json:"modelChanged"`
	FastModeChanged      bool     `json:"fastModeChanged"`
	CacheStrategyChanged bool     `json:"cacheStrategyChanged"`
	EffortChanged        bool     `json:"effortChanged"`
	AddedTools           []string `json:"addedTools,omitempty"`
	RemovedTools         []string `json:"removedTools,omitempty"`
	ChangedToolSchemas   []string `json:"changedToolSchemas,omitempty"`
}

// HasChanges returns true if any changes were detected.
func (c PromptCacheBreakChange) HasChanges() bool {
	return c.SystemPromptChanged ||
		c.ModelChanged ||
		c.FastModeChanged ||
		c.CacheStrategyChanged ||
		c.EffortChanged ||
		len(c.AddedTools) > 0 ||
		len(c.RemovedTools) > 0 ||
		len(c.ChangedToolSchemas) > 0
}

// Summary returns a human-readable description of changes.
func (c PromptCacheBreakChange) Summary() string {
	if !c.HasChanges() {
		return "no changes detected"
	}

	parts := make([]string, 0, 4)
	if c.SystemPromptChanged {
		parts = append(parts, "system prompt changed")
	}
	if c.ModelChanged {
		parts = append(parts, "model changed")
	}
	if len(c.AddedTools) > 0 {
		parts = append(parts, fmt.Sprintf("%d tools added", len(c.AddedTools)))
	}
	if len(c.RemovedTools) > 0 {
		parts = append(parts, fmt.Sprintf("%d tools removed", len(c.RemovedTools)))
	}
	if len(c.ChangedToolSchemas) > 0 {
		parts = append(parts, fmt.Sprintf("%d tool schemas changed", len(c.ChangedToolSchemas)))
	}
	if c.FastModeChanged {
		parts = append(parts, "fast mode changed")
	}
	if c.CacheStrategyChanged {
		parts = append(parts, "cache strategy changed")
	}
	if c.EffortChanged {
		parts = append(parts, "effort changed")
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}

// PromptCacheDiagEvent records a detected cache break with its diagnosis.
type PromptCacheDiagEvent struct {
	Source         string                 `json:"source"`
	PreviousTokens int64                 `json:"previousTokens"`
	CurrentTokens  int64                 `json:"currentTokens"`
	DropPercent    float64               `json:"dropPercent"`
	Changes        PromptCacheBreakChange `json:"changes"`
	Timestamp      time.Time             `json:"timestamp"`
}

// PromptCacheDiagnostics tracks prompt state per source and diagnoses cache breaks.
type PromptCacheDiagnostics struct {
	mu         sync.Mutex
	snapshots  map[string]*PromptStateSnapshot
	baselines  map[string]int64
	events     []PromptCacheDiagEvent
	maxSources int
}

// NewPromptCacheDiagnostics creates a diagnostics tracker with bounded source capacity.
func NewPromptCacheDiagnostics(maxSources int) *PromptCacheDiagnostics {
	if maxSources <= 0 {
		maxSources = 100
	}
	return &PromptCacheDiagnostics{
		snapshots:  make(map[string]*PromptStateSnapshot),
		baselines:  make(map[string]int64),
		maxSources: maxSources,
	}
}

// RecordPreCall saves a snapshot before an API call for a given source.
func (d *PromptCacheDiagnostics) RecordPreCall(source string, snapshot PromptStateSnapshot) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	if _, exists := d.snapshots[source]; !exists && len(d.snapshots) >= d.maxSources {
		d.evictOldest()
	}

	d.snapshots[source] = &snapshot
}

// RecordPostCall checks cache read tokens against baseline and detects breaks.
func (d *PromptCacheDiagnostics) RecordPostCall(source string, cacheReadTokens int64) *PromptCacheDiagEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, hasSnapshot := d.snapshots[source]
	baseline, hasBaseline := d.baselines[source]

	d.baselines[source] = cacheReadTokens

	if !hasBaseline || !hasSnapshot || baseline == 0 {
		return nil
	}

	drop := float64(baseline-cacheReadTokens) / float64(baseline)
	if drop < PromptCacheBreakThreshold {
		return nil
	}

	event := PromptCacheDiagEvent{
		Source:         source,
		PreviousTokens: baseline,
		CurrentTokens:  cacheReadTokens,
		DropPercent:    drop * 100,
		Timestamp:      time.Now(),
	}

	d.events = append(d.events, event)
	return &event
}

// DetectPromptChanges compares two snapshots and returns the differences.
func DetectPromptChanges(prev, curr *PromptStateSnapshot) PromptCacheBreakChange {
	if prev == nil || curr == nil {
		return PromptCacheBreakChange{}
	}

	changes := PromptCacheBreakChange{
		SystemPromptChanged:  prev.SystemPromptHash != curr.SystemPromptHash,
		ModelChanged:         prev.Model != curr.Model,
		FastModeChanged:      prev.FastMode != curr.FastMode,
		CacheStrategyChanged: prev.CacheStrategy != curr.CacheStrategy,
		EffortChanged:        prev.EffortValue != curr.EffortValue,
	}

	for name := range curr.ToolSchemaHashes {
		prevHash, existed := prev.ToolSchemaHashes[name]
		if !existed {
			changes.AddedTools = append(changes.AddedTools, name)
		} else if prevHash != curr.ToolSchemaHashes[name] {
			changes.ChangedToolSchemas = append(changes.ChangedToolSchemas, name)
		}
	}
	for name := range prev.ToolSchemaHashes {
		if _, exists := curr.ToolSchemaHashes[name]; !exists {
			changes.RemovedTools = append(changes.RemovedTools, name)
		}
	}

	return changes
}

// Events returns all recorded diagnostic events.
func (d *PromptCacheDiagnostics) Events() []PromptCacheDiagEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make([]PromptCacheDiagEvent, len(d.events))
	copy(result, d.events)
	return result
}

// ClearEvents removes all recorded events.
func (d *PromptCacheDiagnostics) ClearEvents() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.events = nil
}

// SourceCount returns the number of tracked sources.
func (d *PromptCacheDiagnostics) SourceCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.snapshots)
}

// HashJSONContent returns a SHA-256 hex hash of any JSON-serializable content.
func HashJSONContent(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (d *PromptCacheDiagnostics) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, s := range d.snapshots {
		if oldestKey == "" || s.Timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = s.Timestamp
		}
	}

	if oldestKey != "" {
		delete(d.snapshots, oldestKey)
		delete(d.baselines, oldestKey)
	}
}
