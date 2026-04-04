package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ParseCronExpression ---

func TestParseCronExpression_Valid(t *testing.T) {
	f, err := agentic.ParseCronExpression("*/5 * * * *")
	require.NoError(t, err)
	assert.Equal(t, []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55}, f.Minute)
	assert.Len(t, f.Hour, 24)
	assert.Len(t, f.DayOfMonth, 31)
	assert.Len(t, f.Month, 12)
	assert.Len(t, f.DayOfWeek, 7)
}

func TestParseCronExpression_SpecificTime(t *testing.T) {
	f, err := agentic.ParseCronExpression("30 9 * * 1-5")
	require.NoError(t, err)
	assert.Equal(t, []int{30}, f.Minute)
	assert.Equal(t, []int{9}, f.Hour)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, f.DayOfWeek)
}

func TestParseCronExpression_Range(t *testing.T) {
	f, err := agentic.ParseCronExpression("0 8-17 * * *")
	require.NoError(t, err)
	assert.Equal(t, []int{8, 9, 10, 11, 12, 13, 14, 15, 16, 17}, f.Hour)
}

func TestParseCronExpression_List(t *testing.T) {
	f, err := agentic.ParseCronExpression("0,15,30,45 * * * *")
	require.NoError(t, err)
	assert.Equal(t, []int{0, 15, 30, 45}, f.Minute)
}

func TestParseCronExpression_RangeWithStep(t *testing.T) {
	f, err := agentic.ParseCronExpression("0 0-23/2 * * *")
	require.NoError(t, err)
	assert.Equal(t, []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22}, f.Hour)
}

func TestParseCronExpression_DayOfWeek7IsSunday(t *testing.T) {
	f, err := agentic.ParseCronExpression("0 0 * * 7")
	require.NoError(t, err)
	assert.Equal(t, []int{0}, f.DayOfWeek, "7 should map to 0 (Sunday)")
}

func TestParseCronExpression_DowRangeWith7(t *testing.T) {
	f, err := agentic.ParseCronExpression("0 0 * * 5-7")
	require.NoError(t, err)
	assert.Contains(t, f.DayOfWeek, 0, "7 in range should map to 0")
	assert.Contains(t, f.DayOfWeek, 5)
	assert.Contains(t, f.DayOfWeek, 6)
}

func TestParseCronExpression_InvalidFieldCount(t *testing.T) {
	_, err := agentic.ParseCronExpression("* * *")
	assert.Error(t, err)
}

func TestParseCronExpression_InvalidRange(t *testing.T) {
	_, err := agentic.ParseCronExpression("60 * * * *")
	assert.Error(t, err)
}

func TestParseCronExpression_InvalidStep(t *testing.T) {
	_, err := agentic.ParseCronExpression("*/0 * * * *")
	assert.Error(t, err)
}

func TestParseCronExpression_InvalidChar(t *testing.T) {
	_, err := agentic.ParseCronExpression("abc * * * *")
	assert.Error(t, err)
}

// --- ComputeNextCronRun ---

func TestComputeNextCronRun_EveryMinute(t *testing.T) {
	f, _ := agentic.ParseCronExpression("* * * * *")
	loc := time.UTC
	from := time.Date(2026, 4, 3, 10, 30, 0, 0, loc)
	next := agentic.ComputeNextCronRun(f, from, loc)
	assert.Equal(t, time.Date(2026, 4, 3, 10, 31, 0, 0, loc), next)
}

func TestComputeNextCronRun_SpecificTime(t *testing.T) {
	f, _ := agentic.ParseCronExpression("0 9 * * *")
	loc := time.UTC
	from := time.Date(2026, 4, 3, 10, 0, 0, 0, loc)
	next := agentic.ComputeNextCronRun(f, from, loc)
	// Next 9am is tomorrow
	assert.Equal(t, time.Date(2026, 4, 4, 9, 0, 0, 0, loc), next)
}

func TestComputeNextCronRun_BeforeSpecificTime(t *testing.T) {
	f, _ := agentic.ParseCronExpression("0 9 * * *")
	loc := time.UTC
	from := time.Date(2026, 4, 3, 8, 0, 0, 0, loc)
	next := agentic.ComputeNextCronRun(f, from, loc)
	assert.Equal(t, time.Date(2026, 4, 3, 9, 0, 0, 0, loc), next)
}

func TestComputeNextCronRun_Weekday(t *testing.T) {
	f, _ := agentic.ParseCronExpression("0 9 * * 1") // Monday
	loc := time.UTC
	// April 3, 2026 is a Friday
	from := time.Date(2026, 4, 3, 10, 0, 0, 0, loc)
	next := agentic.ComputeNextCronRun(f, from, loc)
	// Next Monday is April 6
	assert.Equal(t, time.Date(2026, 4, 6, 9, 0, 0, 0, loc), next)
}

func TestComputeNextCronRun_MonthBoundary(t *testing.T) {
	f, _ := agentic.ParseCronExpression("0 0 1 * *") // First of month
	loc := time.UTC
	from := time.Date(2026, 4, 15, 0, 0, 0, 0, loc)
	next := agentic.ComputeNextCronRun(f, from, loc)
	assert.Equal(t, time.Date(2026, 5, 1, 0, 0, 0, 0, loc), next)
}

func TestComputeNextCronRun_StrictlyAfter(t *testing.T) {
	f, _ := agentic.ParseCronExpression("30 10 * * *")
	loc := time.UTC
	// from is exactly on the cron time — should return next day
	from := time.Date(2026, 4, 3, 10, 30, 0, 0, loc)
	next := agentic.ComputeNextCronRun(f, from, loc)
	assert.Equal(t, time.Date(2026, 4, 4, 10, 30, 0, 0, loc), next)
}

// --- CronToHuman ---

func TestCronToHuman_EveryMinute(t *testing.T) {
	assert.Equal(t, "Every minute", agentic.CronToHuman("*/1 * * * *"))
}

func TestCronToHuman_Every5Minutes(t *testing.T) {
	assert.Equal(t, "Every 5 minutes", agentic.CronToHuman("*/5 * * * *"))
}

func TestCronToHuman_EveryHour(t *testing.T) {
	assert.Equal(t, "Every hour", agentic.CronToHuman("0 * * * *"))
}

func TestCronToHuman_EveryHourAt30(t *testing.T) {
	assert.Equal(t, "Every hour at :30", agentic.CronToHuman("30 * * * *"))
}

func TestCronToHuman_Every2Hours(t *testing.T) {
	assert.Equal(t, "Every 2 hours", agentic.CronToHuman("0 */2 * * *"))
}

func TestCronToHuman_DailyAt9AM(t *testing.T) {
	assert.Equal(t, "Every day at 9:00 AM", agentic.CronToHuman("0 9 * * *"))
}

func TestCronToHuman_DailyAt230PM(t *testing.T) {
	assert.Equal(t, "Every day at 2:30 PM", agentic.CronToHuman("30 14 * * *"))
}

func TestCronToHuman_Weekdays(t *testing.T) {
	assert.Equal(t, "Weekdays at 9:00 AM", agentic.CronToHuman("0 9 * * 1-5"))
}

func TestCronToHuman_Monday(t *testing.T) {
	assert.Equal(t, "Every Monday at 9:00 AM", agentic.CronToHuman("0 9 * * 1"))
}

func TestCronToHuman_Sunday(t *testing.T) {
	assert.Equal(t, "Every Sunday at 10:00 AM", agentic.CronToHuman("0 10 * * 0"))
}

func TestCronToHuman_Complex_Fallback(t *testing.T) {
	expr := "0 9 1,15 * *"
	assert.Equal(t, expr, agentic.CronToHuman(expr), "complex expressions fall back to raw string")
}

func TestCronToHuman_Invalid(t *testing.T) {
	assert.Equal(t, "not a cron", agentic.CronToHuman("not a cron"))
}
