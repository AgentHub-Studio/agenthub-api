package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewMetricsStore ---

func TestNewMetricsStore(t *testing.T) {
	m := agentic.NewMetricsStore()
	assert.NotNil(t, m)
	assert.Empty(t, m.GetAll())
}

// --- Increment ---

func TestMetricsStore_Increment(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Increment("requests", 1)
	m.Increment("requests", 1)
	m.Increment("requests", 3)
	assert.Equal(t, 5.0, m.GetCounter("requests"))
}

func TestMetricsStore_Increment_Multiple(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Increment("a", 1)
	m.Increment("b", 2)
	all := m.GetAll()
	assert.Equal(t, 1.0, all["a"])
	assert.Equal(t, 2.0, all["b"])
}

// --- Set (gauge) ---

func TestMetricsStore_Set(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Set("goroutines", 42)
	assert.Equal(t, 42.0, m.GetCounter("goroutines"))

	m.Set("goroutines", 50)
	assert.Equal(t, 50.0, m.GetCounter("goroutines"))
}

// --- Observe (histogram) ---

func TestMetricsStore_Observe(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Observe("latency", 10)
	m.Observe("latency", 20)
	m.Observe("latency", 30)

	assert.Equal(t, 3, m.GetHistogramCount("latency"))

	all := m.GetAll()
	assert.Equal(t, 3.0, all["latency_count"])
	assert.Equal(t, 10.0, all["latency_min"])
	assert.Equal(t, 30.0, all["latency_max"])
	assert.InDelta(t, 20.0, all["latency_avg"], 0.001)
}

func TestMetricsStore_Observe_Percentiles(t *testing.T) {
	m := agentic.NewMetricsStore()
	// Insert 100 values: 1, 2, 3, ..., 100
	for i := 1; i <= 100; i++ {
		m.Observe("latency", float64(i))
	}

	all := m.GetAll()
	assert.InDelta(t, 50.5, all["latency_p50"], 1.0)
	assert.InDelta(t, 95.05, all["latency_p95"], 1.0)
	assert.InDelta(t, 99.01, all["latency_p99"], 1.0)
}

func TestMetricsStore_Observe_SingleValue(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Observe("x", 42)

	all := m.GetAll()
	assert.Equal(t, 42.0, all["x_min"])
	assert.Equal(t, 42.0, all["x_max"])
	assert.Equal(t, 42.0, all["x_avg"])
	assert.Equal(t, 42.0, all["x_p50"])
}

func TestMetricsStore_Observe_ReservoirBounded(t *testing.T) {
	m := agentic.NewMetricsStore()
	// Insert more than reservoir size
	for i := 0; i < 5000; i++ {
		m.Observe("big", float64(i))
	}
	assert.Equal(t, 5000, m.GetHistogramCount("big"))

	// Percentiles should still be reasonable approximations
	all := m.GetAll()
	assert.InDelta(t, 2500, all["big_p50"], 500)
}

// --- Add (cardinality set) ---

func TestMetricsStore_Add(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Add("unique_users", "alice")
	m.Add("unique_users", "bob")
	m.Add("unique_users", "alice") // duplicate

	assert.Equal(t, 2, m.GetSetSize("unique_users"))

	all := m.GetAll()
	assert.Equal(t, 2.0, all["unique_users"])
}

// --- GetHistogramCount not found ---

func TestMetricsStore_GetHistogramCount_NotFound(t *testing.T) {
	m := agentic.NewMetricsStore()
	assert.Equal(t, 0, m.GetHistogramCount("missing"))
}

// --- GetSetSize not found ---

func TestMetricsStore_GetSetSize_NotFound(t *testing.T) {
	m := agentic.NewMetricsStore()
	assert.Equal(t, 0, m.GetSetSize("missing"))
}

// --- GetAll combines all types ---

func TestMetricsStore_GetAll_Combined(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Increment("counter", 5)
	m.Observe("hist", 10)
	m.Add("set", "a")

	all := m.GetAll()
	require.Contains(t, all, "counter")
	require.Contains(t, all, "hist_count")
	require.Contains(t, all, "set")

	assert.Equal(t, 5.0, all["counter"])
	assert.Equal(t, 1.0, all["hist_count"])
	assert.Equal(t, 1.0, all["set"])
}

// --- Reset ---

func TestMetricsStore_Reset(t *testing.T) {
	m := agentic.NewMetricsStore()
	m.Increment("a", 1)
	m.Observe("b", 2)
	m.Add("c", "x")

	m.Reset()
	assert.Empty(t, m.GetAll())
	assert.Equal(t, 0.0, m.GetCounter("a"))
	assert.Equal(t, 0, m.GetHistogramCount("b"))
	assert.Equal(t, 0, m.GetSetSize("c"))
}
