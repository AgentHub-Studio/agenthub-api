package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// ClickHouseSink inserts agentic metrics into ClickHouse JSONEachRow tables.
type ClickHouseSink struct {
	client *ClickHouseClient
}

// NewClickHouseSink creates a sink backed by ClickHouse.
func NewClickHouseSink(client *ClickHouseClient) *ClickHouseSink {
	return &ClickHouseSink{client: client}
}

// InsertRunMetrics inserts a batch of run metrics.
func (s *ClickHouseSink) InsertRunMetrics(metrics []agentic.RunMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, m := range metrics {
		row := map[string]any{
			"tenant_id":         m.TenantID,
			"agent_id":          m.AgentID,
			"session_id":        m.SessionID,
			"run_id":            m.RunID,
			"started_at":        clickhouseTime(nonZeroTime(m.StartedAt)),
			"completed_at":      clickhouseTime(nonZeroTime(m.CompletedAt)),
			"duration_ms":       m.DurationMs,
			"total_turns":       m.TotalTurns,
			"total_tokens":      m.TotalTokens,
			"prompt_tokens":     m.PromptTokens,
			"completion_tokens": m.CompletionTokens,
			"cost_usd":          m.CostUSD,
			"provider":          m.Provider,
			"model":             m.Model,
			"finish_reason":     m.FinishReason,
			"error":             m.Error,
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return s.client.InsertJSONEachRow(context.Background(), "agent_run_metrics", buf.Bytes())
}

// InsertToolMetrics inserts a batch of tool metrics.
func (s *ClickHouseSink) InsertToolMetrics(metrics []agentic.ToolMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, m := range metrics {
		row := map[string]any{
			"tenant_id":   m.TenantID,
			"agent_id":    m.AgentID,
			"session_id":  m.SessionID,
			"run_id":      m.RunID,
			"tool_name":   m.ToolName,
			"tool_id":     m.ToolID,
			"started_at":  clickhouseTime(nonZeroTime(m.StartedAt)),
			"duration_ms": m.DurationMs,
			"state":       m.State,
			"error":       m.Error,
			"was_cached":  m.WasCached,
			"was_denied":  m.WasDenied,
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return s.client.InsertJSONEachRow(context.Background(), "tool_execution_metrics", buf.Bytes())
}

// AsyncSink wraps an AnalyticsSink with a bounded background queue.
type AsyncSink struct {
	sink       agentic.AnalyticsSink
	ch         chan metricBatch
	flushEvery time.Duration
	flushSize  int
}

type metricBatch struct {
	runMetrics  []agentic.RunMetric
	toolMetrics []agentic.ToolMetric
}

// NewAsyncSink creates a non-blocking sink wrapper.
func NewAsyncSink(sink agentic.AnalyticsSink, buffer int) *AsyncSink {
	if buffer <= 0 {
		buffer = 100
	}
	a := &AsyncSink{
		sink:       sink,
		ch:         make(chan metricBatch, buffer),
		flushEvery: 5 * time.Second,
		flushSize:  100,
	}
	go a.loop()
	return a
}

// InsertRunMetrics enqueues run metrics without blocking the caller.
func (a *AsyncSink) InsertRunMetrics(metrics []agentic.RunMetric) error {
	cp := append([]agentic.RunMetric(nil), metrics...)
	return a.enqueue(metricBatch{runMetrics: cp})
}

// InsertToolMetrics enqueues tool metrics without blocking the caller.
func (a *AsyncSink) InsertToolMetrics(metrics []agentic.ToolMetric) error {
	cp := append([]agentic.ToolMetric(nil), metrics...)
	return a.enqueue(metricBatch{toolMetrics: cp})
}

func (a *AsyncSink) enqueue(batch metricBatch) error {
	select {
	case a.ch <- batch:
	default:
		slog.Warn("analytics: clickhouse queue full, dropping metric batch")
	}
	return nil
}

func (a *AsyncSink) loop() {
	ticker := time.NewTicker(a.flushEvery)
	defer ticker.Stop()

	var runMetrics []agentic.RunMetric
	var toolMetrics []agentic.ToolMetric
	flush := func() {
		if len(runMetrics) > 0 {
			if err := a.sink.InsertRunMetrics(runMetrics); err != nil {
				slog.Warn("analytics: clickhouse run insert failed", "err", err)
			}
			runMetrics = nil
		}
		if len(toolMetrics) > 0 {
			if err := a.sink.InsertToolMetrics(toolMetrics); err != nil {
				slog.Warn("analytics: clickhouse tool insert failed", "err", err)
			}
			toolMetrics = nil
		}
	}

	for {
		select {
		case batch, ok := <-a.ch:
			if !ok {
				flush()
				return
			}
			runMetrics = append(runMetrics, batch.runMetrics...)
			toolMetrics = append(toolMetrics, batch.toolMetrics...)
			if len(runMetrics)+len(toolMetrics) >= a.flushSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func nonZeroTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

var _ agentic.AnalyticsSink = (*ClickHouseSink)(nil)
var _ agentic.AnalyticsSink = (*AsyncSink)(nil)
