package agentic

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

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
}

// ToolMetric captures metrics for a single tool execution within a run.
type ToolMetric struct {
	TenantID   string    `json:"tenantId"`
	AgentID    uuid.UUID `json:"agentId"`
	SessionID  uuid.UUID `json:"sessionId"`
	RunID      string    `json:"runId"`
	ToolName   string    `json:"toolName"`
	ToolID     string    `json:"toolId"`
	StartedAt  time.Time `json:"startedAt"`
	DurationMs int64     `json:"durationMs"`
	State      string    `json:"state"`
	Error      *string   `json:"error,omitempty"`
	WasCached  bool      `json:"wasCached"`
	WasDenied  bool      `json:"wasDenied"`
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
	toolStarts   map[string]time.Time // toolID → start time
}

// NewMetricsCollector creates a MetricsCollector for a specific run.
func NewMetricsCollector(sink AnalyticsSink, tenantID string, agentID, sessionID uuid.UUID, runID, provider, model string) *MetricsCollector {
	return &MetricsCollector{
		sink:       sink,
		tenantID:   tenantID,
		agentID:    agentID,
		sessionID:  sessionID,
		runID:      runID,
		provider:   provider,
		model:      model,
		startedAt:  time.Now(),
		toolStarts: make(map[string]time.Time),
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
