package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolRetry_MaxThreeAttempts verifies that the first maxToolRetries calls succeed.
func TestToolRetry_MaxThreeAttempts(t *testing.T) {
	rs := newRunState()

	for i := 0; i < maxToolRetries; i++ {
		err := rs.checkAndIncrementRetry("failing_tool")
		assert.NoError(t, err, "attempt %d should be allowed", i+1)
	}
}

// TestToolRetry_StopsAfterMax verifies that the (maxToolRetries+1)-th call is blocked.
func TestToolRetry_StopsAfterMax(t *testing.T) {
	rs := newRunState()

	for i := 0; i < maxToolRetries; i++ {
		require.NoError(t, rs.checkAndIncrementRetry("failing_tool"))
	}

	err := rs.checkAndIncrementRetry("failing_tool")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum call limit")
	assert.Contains(t, err.Error(), "failing_tool")
}

// TestToolRetry_DifferentTools_IndependentCounters verifies counters are per tool name.
func TestToolRetry_DifferentTools_IndependentCounters(t *testing.T) {
	rs := newRunState()

	// Exhaust retries for tool_a.
	for i := 0; i < maxToolRetries; i++ {
		require.NoError(t, rs.checkAndIncrementRetry("tool_a"))
	}
	assert.Error(t, rs.checkAndIncrementRetry("tool_a"), "tool_a should be exhausted")

	// tool_b should still be available.
	assert.NoError(t, rs.checkAndIncrementRetry("tool_b"))
	assert.NoError(t, rs.checkAndIncrementRetry("tool_b"))
}

// TestToolRetry_CounterIncrementedOnEachCall verifies the counter tracks every invocation.
func TestToolRetry_CounterIncrementedOnEachCall(t *testing.T) {
	rs := newRunState()

	require.NoError(t, rs.checkAndIncrementRetry("my_tool"))
	assert.Equal(t, 1, rs.toolRetries["my_tool"])

	require.NoError(t, rs.checkAndIncrementRetry("my_tool"))
	assert.Equal(t, 2, rs.toolRetries["my_tool"])
}
