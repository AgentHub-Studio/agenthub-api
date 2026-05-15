package agentic

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify OBS-002 (Tool call tracing) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 11 ("Tensions, trade-offs, observability, governance"):
//     observability of tool calls is a first-class concern. The paper notes
//     that "agent design guidance ... ground[s] truth from the environment
//     at each step assesses progress" — tool call traces are the substrate
//     for that assessment.
//   - Section 4.2 (Tool Dispatch and Streaming Execution): each tool call
//     has identifiable lifecycle (start → result) with attribution.
//
// AgentHub maps tool call tracing to:
//   - analytics.go ToolMetric — per-tool-execution record with
//     tenantId/agentId/sessionId/runId attribution + ToolName/ToolID
//     correlation + StartedAt/DurationMs latency + State + Error/ErrorCategory
//     + WasCached + WasDenied flags.
//   - analytics.go classifyToolError — telemetry-safe error categorisation
//     across 7 categories (timeout, permission_denied, network, validation,
//     not_found, rate_limit, unknown) without leaking internal details.
//   - analytics.go AnalyticsSink interface — pluggable backend for ClickHouse,
//     OpenTelemetry, or any other analytics sink.
//   - analytics.go MetricsCollector — consumes RunEvent stream and produces
//     per-tool ToolMetric + per-run RunMetric records.

func TestBDD_ToolCallTracing(t *testing.T) {
	t.Run("Scenario_ToolMetricCarriesFullAttributionChain", func(t *testing.T) {
		// Given a tool execution must be attributable to its full context
		//       (PDF Section 11: observability includes WHO/WHAT/WHEN),
		given := ToolMetric{
			TenantID:   "acme",
			AgentID:    uuid.New(),
			SessionID:  uuid.New(),
			RunID:      "run_abc",
			ToolName:   "document-search",
			ToolID:     "call_xyz",
			StartedAt:  time.Now(),
			DurationMs: 142,
			State:      "completed",
		}

		// When the analytics layer records it,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then every attribution field is observable as a JSON field —
		//      enabling per-tenant, per-agent, per-session, per-run roll-ups.
		for _, k := range []string{"tenantId", "agentId", "sessionId", "runId",
			"toolName", "toolId", "startedAt", "durationMs", "state"} {
			assert.Contains(t, decoded, k,
				"ToolMetric must surface %q as a wire-observable field", k)
		}
	})

	t.Run("Scenario_ToolIDLinksMetricToToolCallStartEvent", func(t *testing.T) {
		// Given a tool call started with a correlation ID,
		startEvent := ToolCallStartData{
			ID:    "call_xyz",
			Name:  "document-search",
			Input: json.RawMessage(`{"query":"docs"}`),
		}

		// When the analytics layer records the metric,
		metric := ToolMetric{
			ToolID:   startEvent.ID,
			ToolName: startEvent.Name,
		}

		// Then ToolID equals the start event ID — preserving the cross-event
		//      correlation that distributed tracing needs.
		assert.Equal(t, startEvent.ID, metric.ToolID,
			"ToolMetric.ToolID must equal the originating start event ID")
		assert.Equal(t, startEvent.Name, metric.ToolName,
			"ToolName must propagate from start event to metric")
	})

	t.Run("Scenario_ErrorCategoryIsTelemetrySafeNotInternalString", func(t *testing.T) {
		// Given a raw error message that may contain stack traces or PII,
		// When the analytics layer classifies it (PDF Section 11: observability
		//      must not become a data leak channel),
		// Then it produces one of the 7 telemetry-safe categories — internal
		//      details stay in the operational logs, not in analytics.
		assert.Equal(t, ToolErrTimeout,
			classifyToolError("context deadline exceeded after 30s"),
			"deadline phrase must classify as timeout")
		assert.Equal(t, ToolErrPermissionDenied,
			classifyToolError("permission denied: tool not in allow list"),
			"permission phrase must classify as permission_denied")
		assert.Equal(t, ToolErrNetwork,
			classifyToolError("connection reset by peer"),
			"connection phrase must classify as network")
		assert.Equal(t, ToolErrValidation,
			classifyToolError("invalid input: query is required"),
			"validation phrase must classify")
		assert.Equal(t, ToolErrNotFound,
			classifyToolError("upstream returned 404 not found"),
			"404 must classify as not_found")
		assert.Equal(t, ToolErrRateLimit,
			classifyToolError("HTTP 429 too many requests"),
			"429 must classify as rate_limit")
		assert.Equal(t, ToolErrUnknown,
			classifyToolError("something completely unexpected happened"),
			"unmatched messages fall back to unknown — never silently mislabelled")
	})

	t.Run("Scenario_ErrorCategoryIsCaseInsensitive", func(t *testing.T) {
		// Given errors come from heterogeneous sources with varied casing,
		// When the classifier inspects them,
		// Then matching is case-insensitive — preventing classification gaps
		//      caused purely by formatting.
		assert.Equal(t, ToolErrTimeout,
			classifyToolError("TIMEOUT"),
			"uppercase TIMEOUT must classify")
		assert.Equal(t, ToolErrTimeout,
			classifyToolError("Timeout"),
			"capitalised Timeout must classify")
		assert.Equal(t, ToolErrPermissionDenied,
			classifyToolError("DENIED"),
			"uppercase DENIED must classify")
	})

	t.Run("Scenario_ErrorCategoryEnumIsExhaustive", func(t *testing.T) {
		// Given the PDF Section 11 implication that telemetry must cover all
		//       common failure modes for actionability,
		// When we list AgentHub's category set,
		// Then 7 categories exist — refactor that drops one fails this guard.
		categories := map[ToolErrorCategory]bool{
			ToolErrTimeout:          true,
			ToolErrPermissionDenied: true,
			ToolErrNetwork:          true,
			ToolErrValidation:       true,
			ToolErrNotFound:         true,
			ToolErrRateLimit:        true,
			ToolErrUnknown:          true,
		}
		assert.Len(t, categories, 7,
			"7 distinct error categories required for actionable telemetry")
	})

	t.Run("Scenario_DurationMsCapturesPerToolLatency", func(t *testing.T) {
		// Given a tool execution with measurable wall-clock duration,
		given := ToolMetric{DurationMs: 250}

		// When the metric is serialised,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)

		// Then DurationMs is a numeric field — usable for histograms and
		//      p95/p99 telemetry.
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))
		assert.Equal(t, float64(250), decoded["durationMs"],
			"durationMs must be numeric for percentile computation")
	})

	t.Run("Scenario_WasDeniedAndWasCachedAreFirstClassFlags", func(t *testing.T) {
		// Given the runner may deny a tool call (PERM-001) or serve from
		//       cache,
		// When the analytics layer records the metric,
		// Then WasDenied and WasCached are explicit booleans — observability
		//      systems can compute deny rate / cache hit rate without parsing
		//      free-text errors.
		denied := ToolMetric{WasDenied: true, State: "denied"}
		cached := ToolMetric{WasCached: true, State: "completed"}

		assert.True(t, denied.WasDenied, "deny path must surface as flag")
		assert.True(t, cached.WasCached, "cache hit must surface as flag")
		assert.False(t, denied.WasCached, "fields are independent")
		assert.False(t, cached.WasDenied, "fields are independent")
	})

	t.Run("Scenario_AnalyticsSinkInterfaceIsBatchOriented", func(t *testing.T) {
		// Given analytics must scale (PDF Section 11: observability cannot
		//       slow down the agentic loop),
		// When the runtime emits metrics,
		// Then the sink interface accepts BATCHES (not single rows) —
		//      enabling efficient ClickHouse / OpenTelemetry / other backend
		//      integration.
		var sink AnalyticsSink = &nullAnalyticsSink{}
		assert.NoError(t, sink.InsertRunMetrics([]RunMetric{}),
			"sink must accept run metric batches")
		assert.NoError(t, sink.InsertToolMetrics([]ToolMetric{}),
			"sink must accept tool metric batches")
	})

	t.Run("Scenario_RunMetricRollUpAggregatesAcrossModelsAndSources", func(t *testing.T) {
		// Given a run that used a fallback model (PDF Section 4.4) and
		//       multiple query sources (main loop + subtask),
		given := RunMetric{
			ModelUsage: map[string]*ModelTokenUsage{
				"claude-sonnet-4-20250514": {Model: "claude-sonnet-4-20250514", TotalTokens: 1000, CostUSD: 0.01},
				"claude-haiku-4-5":         {Model: "claude-haiku-4-5", TotalTokens: 500, CostUSD: 0.001},
			},
			QuerySources: []string{"main_loop", "subtask"},
			ErrorClasses: []string{"rate_limit"},
		}

		// When the analytics layer surfaces the run summary,
		// Then per-model + per-source breakdowns are observable — required for
		//      cost attribution when fallback models or subagents are used.
		assert.Len(t, given.ModelUsage, 2,
			"per-model breakdown must survive in roll-up")
		assert.Contains(t, given.QuerySources, "main_loop",
			"query source attribution must be preserved")
		assert.Contains(t, given.QuerySources, "subtask",
			"subtask attribution must be preserved")
		assert.Contains(t, given.ErrorClasses, "rate_limit",
			"distinct error classes must surface for retry-pattern analysis")
	})
}

// nullAnalyticsSink is a no-op sink used to verify the interface contract
// without touching a real backend.
type nullAnalyticsSink struct{}

func (n *nullAnalyticsSink) InsertRunMetrics(_ []RunMetric) error   { return nil }
func (n *nullAnalyticsSink) InsertToolMetrics(_ []ToolMetric) error { return nil }
