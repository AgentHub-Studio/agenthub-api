package agentic

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ModelTokenUsage tracks token usage for a specific model within a run.
// This enables per-model cost tracking when fallback models are used.
type ModelTokenUsage struct {
	Model            string `json:"model"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
	CacheReadTokens  int    `json:"cacheReadTokens,omitempty"`
	CostUSD          float64 `json:"costUsd"`
	Turns            int    `json:"turns"`
}

// RunMetric captures aggregate metrics for a complete agentic run.
type RunMetric struct {
	TenantID         string    `json:"tenantId"`
	AgentID          uuid.UUID `json:"agentId"`
	SessionID        uuid.UUID `json:"sessionId"`
	RunID            string    `json:"runId"`
	StartedAt        time.Time `json:"startedAt"`
	CompletedAt      time.Time `json:"completedAt"`
	DurationMs       int64     `json:"durationMs"`
	TotalTurns       int       `json:"totalTurns"`
	TotalTokens      int       `json:"totalTokens"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	CostUSD          float64   `json:"costUsd"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	FinishReason     string    `json:"finishReason"`
	Error            *string   `json:"error,omitempty"`
	// ModelUsage tracks per-model token usage when fallback models are used.
	// Key is the model name.
	ModelUsage map[string]*ModelTokenUsage `json:"modelUsage,omitempty"`
	// QuerySources lists the distinct execution sources observed during the run
	// (e.g. ["main_loop", "subtask"]). Useful for analytics segmentation.
	QuerySources []string `json:"querySources,omitempty"`
	// ErrorClasses lists the distinct API error classes observed during the run
	// (e.g. ["rate_limit", "stale_connection"]). Useful for debugging retry patterns.
	ErrorClasses []string `json:"errorClasses,omitempty"`
	// CacheBreaks lists detected prompt cache breaks during the run.
	// Each entry describes when and why the server-side cache was invalidated.
	CacheBreaks []CacheBreakEvent `json:"cacheBreaks,omitempty"`
}

// ToolMetric captures metrics for a single tool execution within a run.
type ToolMetric struct {
	TenantID      string            `json:"tenantId"`
	AgentID       uuid.UUID         `json:"agentId"`
	SessionID     uuid.UUID         `json:"sessionId"`
	RunID         string            `json:"runId"`
	ToolName      string            `json:"toolName"`
	ToolID        string            `json:"toolId"`
	StartedAt     time.Time         `json:"startedAt"`
	DurationMs    int64             `json:"durationMs"`
	State         string            `json:"state"`
	Error         *string           `json:"error,omitempty"`
	ErrorCategory ToolErrorCategory `json:"errorCategory,omitempty"`
	WasCached     bool              `json:"wasCached"`
	WasDenied     bool              `json:"wasDenied"`
}

// AnalyticsSink receives batched analytics data.
type AnalyticsSink interface {
	// InsertRunMetrics inserts a batch of run metrics.
	InsertRunMetrics(metrics []RunMetric) error
	// InsertToolMetrics inserts a batch of tool metrics.
	InsertToolMetrics(metrics []ToolMetric) error
}

// MetricsCollector consumes RunEvents and accumulates metrics for a single run.
// It is designed to be used as a goroutine that reads from the event channel.
type MetricsCollector struct {
	mu           sync.Mutex
	sink         AnalyticsSink
	tenantID     string
	agentID      uuid.UUID
	sessionID    uuid.UUID
	runID        string
	provider     string
	model        string
	startedAt    time.Time
	toolMetrics  []ToolMetric
	runMetric    RunMetric
	toolStarts     map[string]time.Time          // toolID → start time
	modelUsage     map[string]*ModelTokenUsage    // model → accumulated usage
	cacheDetector  *CacheBreakDetector
	lastTurnTime   time.Time                      // timestamp of previous turn for TTL gap calc
}

// NewMetricsCollector creates a MetricsCollector for a specific run.
func NewMetricsCollector(sink AnalyticsSink, tenantID string, agentID, sessionID uuid.UUID, runID, provider, model string) *MetricsCollector {
	return &MetricsCollector{
		sink:          sink,
		tenantID:      tenantID,
		agentID:       agentID,
		sessionID:     sessionID,
		runID:         runID,
		provider:      provider,
		model:         model,
		startedAt:     time.Now(),
		toolStarts:    make(map[string]time.Time),
		modelUsage:    make(map[string]*ModelTokenUsage),
		cacheDetector: NewCacheBreakDetector(),
	}
}

// Collect processes a single RunEvent and accumulates metrics.
func (c *MetricsCollector) Collect(event RunEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch event.Type {
	case EventToolCallStart:
		var data ToolCallStartData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			c.toolStarts[data.ID] = time.Now()
		}

	case EventToolProgress:
		var data ToolProgressData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			if data.State == ToolStateCompleted || data.State == ToolStateAborted {
				started := c.toolStarts[data.ID]
				duration := int64(0)
				if !started.IsZero() {
					duration = time.Since(started).Milliseconds()
				}
				c.toolMetrics = append(c.toolMetrics, ToolMetric{
					TenantID:   c.tenantID,
					AgentID:    c.agentID,
					SessionID:  c.sessionID,
					RunID:      c.runID,
					ToolName:   data.Name,
					ToolID:     data.ID,
					StartedAt:  started,
					DurationMs: duration,
					State:      string(data.State),
				})
				delete(c.toolStarts, data.ID)
			}
		}

	case EventToolResult:
		var data ToolResultData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			// Update the last tool metric with result details.
			for i := len(c.toolMetrics) - 1; i >= 0; i-- {
				if c.toolMetrics[i].ToolID == data.ID {
					c.toolMetrics[i].DurationMs = data.DurationMs
					c.toolMetrics[i].Error = data.Error
					if data.Error != nil {
						c.toolMetrics[i].ErrorCategory = classifyToolError(*data.Error)
					}
					break
				}
			}
		}

	case EventToolDenied:
		var data ToolDeniedData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			c.toolMetrics = append(c.toolMetrics, ToolMetric{
				TenantID:  c.tenantID,
				AgentID:   c.agentID,
				SessionID: c.sessionID,
				RunID:     c.runID,
				ToolName:  data.Name,
				ToolID:    data.ID,
				StartedAt: time.Now(),
				State:     "denied",
				WasDenied: true,
			})
		}

	case EventTurnComplete:
		var data TurnCompleteData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			c.runMetric.TotalTurns = data.TurnIndex + 1
			c.runMetric.TotalTokens += data.TokenUsage.TotalTokens
			c.runMetric.PromptTokens += data.TokenUsage.PromptTokens
			c.runMetric.CompletionTokens += data.TokenUsage.CompletionTokens
			c.runMetric.CostUSD += data.TokenUsage.CostUSD

			// Track query source distribution for analytics.
			if data.Source != "" {
				c.runMetric.QuerySources = appendUnique(c.runMetric.QuerySources, string(data.Source))
			}

			// Per-model usage tracking (important for fallback scenarios).
			modelName := data.Model
			if modelName == "" {
				modelName = c.model
			}
			mu, ok := c.modelUsage[modelName]
			if !ok {
				mu = &ModelTokenUsage{Model: modelName}
				c.modelUsage[modelName] = mu
			}
			mu.PromptTokens += data.TokenUsage.PromptTokens
			mu.CompletionTokens += data.TokenUsage.CompletionTokens
			mu.TotalTokens += data.TokenUsage.TotalTokens
			mu.CacheReadTokens += data.TokenUsage.CacheReadTokens
			mu.CostUSD += data.TokenUsage.CostUSD
			mu.Turns++

			// Check for prompt cache breaks (Anthropic-specific).
			if data.TokenUsage.CacheReadTokens > 0 || (c.lastTurnTime != (time.Time{}) && !c.lastTurnTime.IsZero()) {
				timeSinceLast := time.Duration(0)
				if !c.lastTurnTime.IsZero() {
					timeSinceLast = time.Since(c.lastTurnTime)
				}
				snap := CacheBreakSnapshot{
					Source:              string(data.Source),
					Model:               modelName,
					CacheReadTokens:     data.TokenUsage.CacheReadTokens,
					CacheCreationTokens: data.TokenUsage.CacheCreationTokens,
				}
				if brk := c.cacheDetector.CheckForBreak(snap, timeSinceLast); brk != nil {
					c.runMetric.CacheBreaks = append(c.runMetric.CacheBreaks, *brk)
				}
			}
			c.lastTurnTime = time.Now()
		}

	case EventRunComplete:
		var data RunCompleteData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			c.runMetric.TotalTurns = data.TotalTurns
			c.runMetric.TotalTokens = data.TotalTokens
			c.runMetric.CostUSD = data.TotalCost
			c.runMetric.FinishReason = "stop"
		}

	case EventError:
		var data ErrorData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			errMsg := data.Message
			c.runMetric.Error = &errMsg
			c.runMetric.FinishReason = "error"
			// Classify the error for analytics.
			errClass := classifyAPIError(fmt.Errorf("%s", data.Message))
			if errClass != ErrorClassNone && errClass != ErrorClassUnknown {
				c.runMetric.ErrorClasses = appendUnique(c.runMetric.ErrorClasses, string(errClass))
			}
		}
	}
}

// Flush sends accumulated metrics to the sink.
func (c *MetricsCollector) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.runMetric.TenantID = c.tenantID
	c.runMetric.AgentID = c.agentID
	c.runMetric.SessionID = c.sessionID
	c.runMetric.RunID = c.runID
	c.runMetric.Provider = c.provider
	c.runMetric.Model = c.model
	c.runMetric.StartedAt = c.startedAt
	c.runMetric.CompletedAt = now
	c.runMetric.DurationMs = now.Sub(c.startedAt).Milliseconds()

	if c.runMetric.FinishReason == "" {
		c.runMetric.FinishReason = "unknown"
	}

	// Attach per-model usage if more than one model was used.
	if len(c.modelUsage) > 1 {
		c.runMetric.ModelUsage = c.modelUsage
	}

	if err := c.sink.InsertRunMetrics([]RunMetric{c.runMetric}); err != nil {
		return err
	}

	if len(c.toolMetrics) > 0 {
		if err := c.sink.InsertToolMetrics(c.toolMetrics); err != nil {
			return err
		}
	}

	return nil
}

// --- Tool Error Classification ---

// ToolErrorCategory classifies tool execution errors for telemetry.
type ToolErrorCategory string

const (
	ToolErrTimeout          ToolErrorCategory = "timeout"
	ToolErrPermissionDenied ToolErrorCategory = "permission_denied"
	ToolErrNetwork          ToolErrorCategory = "network"
	ToolErrValidation       ToolErrorCategory = "validation"
	ToolErrNotFound         ToolErrorCategory = "not_found"
	ToolErrRateLimit        ToolErrorCategory = "rate_limit"
	ToolErrUnknown          ToolErrorCategory = "unknown"
)

// classifyToolError maps a tool error string into a telemetry-safe category.
// Inspired by Claude Code's classifyToolError — avoids leaking internal error
// details into analytics while still providing actionable categorization.
func classifyToolError(errMsg string) ToolErrorCategory {
	lower := strings.ToLower(errMsg)

	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return ToolErrTimeout
	case strings.Contains(lower, "permission") || strings.Contains(lower, "denied") || strings.Contains(lower, "forbidden"):
		return ToolErrPermissionDenied
	case strings.Contains(lower, "connection") || strings.Contains(lower, "network") || strings.Contains(lower, "dns") || strings.Contains(lower, "eof"):
		return ToolErrNetwork
	case strings.Contains(lower, "validation") || strings.Contains(lower, "invalid") || strings.Contains(lower, "required"):
		return ToolErrValidation
	case strings.Contains(lower, "not found") || strings.Contains(lower, "404"):
		return ToolErrNotFound
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") || strings.Contains(lower, "throttle"):
		return ToolErrRateLimit
	default:
		return ToolErrUnknown
	}
}

// appendUnique appends s to the slice only if it's not already present.
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// --- Prompt Cache Break Detection ---

// CacheBreakDetector tracks cache_read_tokens across LLM calls and detects
// significant drops that indicate the server-side prompt cache was invalidated.
// Inspired by Claude Code's promptCacheBreakDetection.ts — adapted for
// AgentHub's multi-tenant, multi-source architecture.
type CacheBreakDetector struct {
	mu    sync.Mutex
	state map[string]*cacheTrackingState // source → state
}

// cacheTrackingState holds the per-source tracking state for cache break detection.
type cacheTrackingState struct {
	prevCacheReadTokens *int
	prevModel           string
	prevToolCount       int
	prevSystemHash      uint32
	callCount           int
	compactPending      bool // compaction legitimately drops cache — skip next check
}

// CacheBreakEvent holds information about a detected cache break.
type CacheBreakEvent struct {
	Source              string `json:"source"`
	PrevCacheReadTokens int    `json:"prevCacheReadTokens"`
	CacheReadTokens     int    `json:"cacheReadTokens"`
	CacheCreationTokens int    `json:"cacheCreationTokens"`
	TokenDrop           int    `json:"tokenDrop"`
	DropPercent         float64 `json:"dropPercent"`
	Reason              string `json:"reason"`
	CallNumber          int    `json:"callNumber"`
}

// CacheBreakSnapshot captures the state of a single LLM call for cache break comparison.
type CacheBreakSnapshot struct {
	Source              string
	Model               string
	ToolCount           int
	SystemPromptHash    uint32
	CacheReadTokens     int
	CacheCreationTokens int
}

// minCacheMissTokens is the minimum absolute token drop required to trigger detection.
// Small drops can happen due to normal variation and aren't worth alerting on.
const minCacheMissTokens = 2000

// cacheTTL5Min is the 5-minute TTL threshold for Anthropic's prompt cache.
const cacheTTL5Min = 5 * time.Minute

// cacheTTL1Hour is the 1-hour TTL threshold for extended prompt cache.
const cacheTTL1Hour = time.Hour

// NewCacheBreakDetector creates a new detector instance.
func NewCacheBreakDetector() *CacheBreakDetector {
	return &CacheBreakDetector{
		state: make(map[string]*cacheTrackingState),
	}
}

// CheckForBreak examines a post-call snapshot and returns a CacheBreakEvent
// if a significant cache drop was detected, or nil if everything looks normal.
// timeSinceLastCall is used to distinguish TTL expiry from client-side changes.
func (d *CacheBreakDetector) CheckForBreak(snap CacheBreakSnapshot, timeSinceLastCall time.Duration) *CacheBreakEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	st, ok := d.state[snap.Source]
	if !ok {
		d.state[snap.Source] = &cacheTrackingState{
			prevCacheReadTokens: &snap.CacheReadTokens,
			prevModel:           snap.Model,
			prevToolCount:       snap.ToolCount,
			prevSystemHash:      snap.SystemPromptHash,
			callCount:           1,
		}
		return nil
	}

	st.callCount++

	// If compaction just happened, reset baseline and skip detection.
	if st.compactPending {
		st.compactPending = false
		st.prevCacheReadTokens = &snap.CacheReadTokens
		st.prevModel = snap.Model
		st.prevToolCount = snap.ToolCount
		st.prevSystemHash = snap.SystemPromptHash
		return nil
	}

	// First call for this source — no baseline to compare.
	if st.prevCacheReadTokens == nil {
		st.prevCacheReadTokens = &snap.CacheReadTokens
		st.prevModel = snap.Model
		st.prevToolCount = snap.ToolCount
		st.prevSystemHash = snap.SystemPromptHash
		return nil
	}

	prev := *st.prevCacheReadTokens

	// Update state for next call.
	defer func() {
		st.prevCacheReadTokens = &snap.CacheReadTokens
		st.prevModel = snap.Model
		st.prevToolCount = snap.ToolCount
		st.prevSystemHash = snap.SystemPromptHash
	}()

	// Detect cache break: >5% drop AND absolute drop exceeds minimum threshold.
	tokenDrop := prev - snap.CacheReadTokens
	if snap.CacheReadTokens >= int(float64(prev)*0.95) || tokenDrop < minCacheMissTokens {
		return nil
	}

	// Build explanation.
	reason := d.explainBreak(st, snap, timeSinceLastCall)

	dropPct := 0.0
	if prev > 0 {
		dropPct = float64(tokenDrop) / float64(prev) * 100
	}

	return &CacheBreakEvent{
		Source:              snap.Source,
		PrevCacheReadTokens: prev,
		CacheReadTokens:     snap.CacheReadTokens,
		CacheCreationTokens: snap.CacheCreationTokens,
		TokenDrop:           tokenDrop,
		DropPercent:         dropPct,
		Reason:              reason,
		CallNumber:          st.callCount,
	}
}

// explainBreak determines the likely cause of a cache break.
func (d *CacheBreakDetector) explainBreak(st *cacheTrackingState, snap CacheBreakSnapshot, timeSinceLastCall time.Duration) string {
	var parts []string

	if snap.Model != st.prevModel {
		parts = append(parts, fmt.Sprintf("model changed (%s → %s)", st.prevModel, snap.Model))
	}
	if snap.SystemPromptHash != st.prevSystemHash {
		parts = append(parts, "system prompt changed")
	}
	if snap.ToolCount != st.prevToolCount {
		parts = append(parts, fmt.Sprintf("tool count changed (%d → %d)", st.prevToolCount, snap.ToolCount))
	}

	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}

	// No client-side changes — check TTL expiry.
	if timeSinceLastCall > cacheTTL1Hour {
		return "possible 1h TTL expiry (prompt unchanged)"
	}
	if timeSinceLastCall > cacheTTL5Min {
		return "possible 5min TTL expiry (prompt unchanged)"
	}
	if timeSinceLastCall > 0 {
		return "likely server-side (prompt unchanged, <5min gap)"
	}

	return "unknown cause"
}

// NotifyCompaction resets the baseline for a source after context compaction.
// Compaction legitimately reduces message count, so cache reads will naturally
// drop on the next call — that's not a break.
func (d *CacheBreakDetector) NotifyCompaction(source string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if st, ok := d.state[source]; ok {
		st.compactPending = true
	}
}

// Reset clears all tracking state (e.g., on session reset).
func (d *CacheBreakDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state = make(map[string]*cacheTrackingState)
}

// Djb2Hash computes a simple DJB2 hash of the input string.
// Used for fast, non-cryptographic comparison of system prompts across turns.
func Djb2Hash(s string) uint32 {
	var hash uint32 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint32(s[i])
	}
	return hash
}

// CollectFromChannel reads events from a channel and collects metrics.
// It flushes when the channel is closed. This is designed to run in a goroutine.
func (c *MetricsCollector) CollectFromChannel(events <-chan RunEvent) {
	for event := range events {
		c.Collect(event)
	}
	_ = c.Flush()
}

// RunMetrics returns the accumulated run metric (for testing).
func (c *MetricsCollector) RunMetrics() RunMetric {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runMetric
}

// ToolMetrics returns the accumulated tool metrics (for testing).
func (c *MetricsCollector) ToolMetrics() []ToolMetric {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ToolMetric{}, c.toolMetrics...)
}
