package trigger_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/trigger"
)

func TestSimpleCronParser_Validate_Valid(t *testing.T) {
	p := trigger.NewSimpleCronParser()
	assert.NoError(t, p.Validate("0 9 * * *"))
	assert.NoError(t, p.Validate("*/5 * * * *"))
	assert.NoError(t, p.Validate("30 14 * * 1"))
}

func TestSimpleCronParser_Validate_Invalid(t *testing.T) {
	p := trigger.NewSimpleCronParser()
	assert.Error(t, p.Validate("0 9 * *"))    // 4 fields
	assert.Error(t, p.Validate(""))            // empty
	assert.Error(t, p.Validate("0 9 * * * *")) // 6 fields
}

func TestSimpleCronParser_NextRun_EveryHour(t *testing.T) {
	p := trigger.NewSimpleCronParser()
	from := time.Date(2026, 4, 3, 10, 30, 0, 0, time.UTC)
	next, err := p.NextRun("0 * * * *", from)
	require.NoError(t, err)
	assert.Equal(t, 0, next.Minute())
	assert.True(t, next.After(from))
}

func TestSimpleCronParser_NextRun_Every5Min(t *testing.T) {
	p := trigger.NewSimpleCronParser()
	from := time.Date(2026, 4, 3, 10, 7, 0, 0, time.UTC)
	next, err := p.NextRun("*/5 * * * *", from)
	require.NoError(t, err)
	assert.Equal(t, 10, next.Minute())
	assert.True(t, next.After(from))
}

func TestSimpleCronParser_NextRun_SpecificTime(t *testing.T) {
	p := trigger.NewSimpleCronParser()
	from := time.Date(2026, 4, 3, 8, 0, 0, 0, time.UTC)
	next, err := p.NextRun("30 9 * * *", from)
	require.NoError(t, err)
	assert.Equal(t, 30, next.Minute())
	assert.Equal(t, 9, next.Hour())
}

func TestSimpleCronParser_NextRun_InvalidExpr(t *testing.T) {
	p := trigger.NewSimpleCronParser()
	_, err := p.NextRun("invalid", time.Now())
	require.Error(t, err)
}
