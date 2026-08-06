package agentic

import (
	"math"
	"sort"
	"sync"

	"github.com/AgentHub-Studio/agenthub-api/internal/randutil"
)

// Metrics store with reservoir sampling for streaming histograms.
//
// Inspired by Claude Code's createStatsStore — supports counters, gauges,
// histograms (with p50/p95/p99 via reservoir sampling), and cardinality
// sets. Memory-bounded: histograms keep at most ReservoirSize samples
// using Algorithm R, giving accurate percentile estimates without storing
// all observations.

const (
	// MetricsReservoirSize is the max samples kept per histogram.
	MetricsReservoirSize = 1024
)

type histogram struct {
	reservoir []float64
	count     int
	sum       float64
	min       float64
	max       float64
}

// MetricsStore provides counters, gauges, histograms, and cardinality sets.
type MetricsStore struct {
	mu         sync.Mutex
	counters   map[string]float64
	histograms map[string]*histogram
	sets       map[string]map[string]struct{}
}

// NewMetricsStore creates an empty metrics store.
func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		counters:   make(map[string]float64),
		histograms: make(map[string]*histogram),
		sets:       make(map[string]map[string]struct{}),
	}
}

// Increment adds value to a counter (default 1).
func (m *MetricsStore) Increment(name string, value float64) {
	m.mu.Lock()
	m.counters[name] += value
	m.mu.Unlock()
}

// Set sets a gauge to an exact value.
func (m *MetricsStore) Set(name string, value float64) {
	m.mu.Lock()
	m.counters[name] = value
	m.mu.Unlock()
}

// Observe records a histogram observation using reservoir sampling.
func (m *MetricsStore) Observe(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, ok := m.histograms[name]
	if !ok {
		h = &histogram{
			min: value,
			max: value,
		}
		m.histograms[name] = h
	}

	h.count++
	h.sum += value
	if value < h.min {
		h.min = value
	}
	if value > h.max {
		h.max = value
	}

	// Algorithm R reservoir sampling
	if len(h.reservoir) < MetricsReservoirSize {
		h.reservoir = append(h.reservoir, value)
	} else {
		j := randutil.Intn(h.count)
		if j < MetricsReservoirSize {
			h.reservoir[j] = value
		}
	}
}

// Add records a cardinality set member. GetAll reports the set size.
func (m *MetricsStore) Add(name string, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sets[name]
	if !ok {
		s = make(map[string]struct{})
		m.sets[name] = s
	}
	s[value] = struct{}{}
}

// GetAll returns a flat map of all metrics including histogram aggregates.
func (m *MetricsStore) GetAll() map[string]float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]float64, len(m.counters))
	for k, v := range m.counters {
		result[k] = v
	}

	for name, h := range m.histograms {
		if h.count == 0 {
			continue
		}
		result[name+"_count"] = float64(h.count)
		result[name+"_min"] = h.min
		result[name+"_max"] = h.max
		result[name+"_avg"] = h.sum / float64(h.count)

		sorted := make([]float64, len(h.reservoir))
		copy(sorted, h.reservoir)
		sort.Float64s(sorted)

		result[name+"_p50"] = percentileCalc(sorted, 50)
		result[name+"_p95"] = percentileCalc(sorted, 95)
		result[name+"_p99"] = percentileCalc(sorted, 99)
	}

	for name, s := range m.sets {
		result[name] = float64(len(s))
	}

	return result
}

// GetCounter returns a counter value.
func (m *MetricsStore) GetCounter(name string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[name]
}

// GetHistogramCount returns the observation count for a histogram.
func (m *MetricsStore) GetHistogramCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.histograms[name]
	if !ok {
		return 0
	}
	return h.count
}

// GetSetSize returns the cardinality of a set.
func (m *MetricsStore) GetSetSize(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sets[name])
}

// Reset clears all metrics.
func (m *MetricsStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters = make(map[string]float64)
	m.histograms = make(map[string]*histogram)
	m.sets = make(map[string]map[string]struct{})
}

// percentileCalc computes the p-th percentile from a sorted slice.
func percentileCalc(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower] + (sorted[upper]-sorted[lower])*frac
}
