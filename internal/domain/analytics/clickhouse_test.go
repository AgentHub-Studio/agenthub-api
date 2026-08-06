package analytics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

type clickHouseRequest struct {
	database string
	body     string
}

func newClickHouseTestServer(t *testing.T, handler func(http.ResponseWriter, *http.Request, string)) (*ClickHouseClient, *[]clickHouseRequest) {
	t.Helper()
	var mu sync.Mutex
	var requests []clickHouseRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		mu.Lock()
		requests = append(requests, clickHouseRequest{
			database: r.URL.Query().Get("database"),
			body:     string(body),
		})
		mu.Unlock()
		handler(w, r, string(body))
	}))
	t.Cleanup(ts.Close)

	client := NewClickHouseClient(config.ClickHouseConfig{
		URL:      ts.URL,
		Database: "agenthub",
	})
	return client, &requests
}

func TestClickHouseClient_EnsureSchemaCreatesAnalyticsTables(t *testing.T) {
	client, requests := newClickHouseTestServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusOK)
	})

	err := client.EnsureSchema(context.Background())
	require.NoError(t, err)
	require.Len(t, *requests, 3)

	assert.Equal(t, "", (*requests)[0].database)
	assert.Contains(t, (*requests)[0].body, "CREATE DATABASE IF NOT EXISTS `agenthub`")
	assert.Equal(t, "agenthub", (*requests)[1].database)
	assert.Contains(t, (*requests)[1].body, "CREATE TABLE IF NOT EXISTS agent_run_metrics")
	assert.Equal(t, "agenthub", (*requests)[2].database)
	assert.Contains(t, (*requests)[2].body, "CREATE TABLE IF NOT EXISTS tool_execution_metrics")
}

func TestClickHouseSink_InsertRunAndToolMetrics(t *testing.T) {
	client, requests := newClickHouseTestServer(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusOK)
	})
	sink := NewClickHouseSink(client)
	agentID := uuid.New()
	sessionID := uuid.New()
	started := time.Date(2026, 6, 20, 10, 30, 0, 123000000, time.UTC)

	err := sink.InsertRunMetrics([]agentic.RunMetric{{
		TenantID:         "tenant",
		AgentID:          agentID,
		SessionID:        sessionID,
		RunID:            "run-1",
		StartedAt:        started,
		CompletedAt:      started.Add(2 * time.Second),
		DurationMs:       2000,
		TotalTurns:       2,
		TotalTokens:      150,
		PromptTokens:     100,
		CompletionTokens: 50,
		CostUSD:          0.012,
		Provider:         "anthropic",
		Model:            "claude-sonnet",
		FinishReason:     "stop",
	}})
	require.NoError(t, err)

	err = sink.InsertToolMetrics([]agentic.ToolMetric{{
		TenantID:   "tenant",
		AgentID:    agentID,
		SessionID:  sessionID,
		RunID:      "run-1",
		ToolName:   "document_search",
		ToolID:     "tool-1",
		StartedAt:  started,
		DurationMs: 42,
		State:      "completed",
	}})
	require.NoError(t, err)
	require.Len(t, *requests, 2)

	assert.Contains(t, (*requests)[0].body, "INSERT INTO `agent_run_metrics` FORMAT JSONEachRow")
	assert.Contains(t, (*requests)[0].body, `"tenant_id":"tenant"`)
	assert.Contains(t, (*requests)[0].body, `"model":"claude-sonnet"`)
	assert.Contains(t, (*requests)[1].body, "INSERT INTO `tool_execution_metrics` FORMAT JSONEachRow")
	assert.Contains(t, (*requests)[1].body, `"tool_name":"document_search"`)
}

func TestClickHouseStore_AgentUsage(t *testing.T) {
	agentID := uuid.New()
	client, requests := newClickHouseTestServer(t, func(w http.ResponseWriter, _ *http.Request, body string) {
		require.Contains(t, body, "FROM agent_run_metrics")
		require.Contains(t, body, "FORMAT JSONEachRow")
		_, _ = io.WriteString(w, `{"totalRuns":2,"totalTokens":300,"promptTokens":200,"completionTokens":100,"totalCostUsd":0.12,"avgDurationMs":1500,"avgTurns":3}`+"\n")
	})
	store := NewClickHouseStore(client)

	out, err := store.AgentUsage(context.Background(), agentID, TimeRange{
		From: time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	assert.Equal(t, agentID, out.AgentID)
	assert.Equal(t, 2, out.TotalRuns)
	assert.Equal(t, int64(300), out.TotalTokens)
	assert.InDelta(t, 0.12, out.TotalCostUSD, 0.0001)
	require.Len(t, *requests, 1)
	assert.Equal(t, "agenthub", (*requests)[0].database)
	assert.True(t, strings.Contains((*requests)[0].body, agentID.String()))
}

type analyticsSinkSpy struct {
	mu          sync.Mutex
	runBatches  [][]agentic.RunMetric
	toolBatches [][]agentic.ToolMetric
}

func (s *analyticsSinkSpy) InsertRunMetrics(metrics []agentic.RunMetric) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runBatches = append(s.runBatches, append([]agentic.RunMetric(nil), metrics...))
	return nil
}

func (s *analyticsSinkSpy) InsertToolMetrics(metrics []agentic.ToolMetric) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolBatches = append(s.toolBatches, append([]agentic.ToolMetric(nil), metrics...))
	return nil
}

func TestAsyncSink_FlushesWhenBatchLimitIsReached(t *testing.T) {
	spy := &analyticsSinkSpy{}
	async := &AsyncSink{
		sink:       spy,
		ch:         make(chan metricBatch, 10),
		flushEvery: time.Hour,
		flushSize:  2,
	}
	go async.loop()

	err := async.InsertRunMetrics([]agentic.RunMetric{{RunID: "run-1"}})
	require.NoError(t, err)
	err = async.InsertToolMetrics([]agentic.ToolMetric{{RunID: "run-1", ToolName: "search"}})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		spy.mu.Lock()
		defer spy.mu.Unlock()
		return len(spy.runBatches) == 1 && len(spy.toolBatches) == 1
	}, time.Second, 10*time.Millisecond)
}
