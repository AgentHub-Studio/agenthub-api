package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

var refTime = time.Date(2026, 4, 3, 14, 30, 0, 0, time.UTC)

func TestFormatBriefTimestamp_SameDay(t *testing.T) {
	ts := time.Date(2026, 4, 3, 10, 15, 0, 0, time.UTC)
	result := agentic.FormatBriefTimestamp(ts, refTime)
	assert.Equal(t, "10:15", result)
}

func TestFormatBriefTimestamp_SameDayLater(t *testing.T) {
	ts := time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC)
	result := agentic.FormatBriefTimestamp(ts, refTime)
	assert.Equal(t, "14:00", result)
}

func TestFormatBriefTimestamp_Yesterday(t *testing.T) {
	ts := time.Date(2026, 4, 2, 16, 45, 0, 0, time.UTC)
	result := agentic.FormatBriefTimestamp(ts, refTime)
	assert.Contains(t, result, "Thursday")
	assert.Contains(t, result, "16:45")
}

func TestFormatBriefTimestamp_ThreeDaysAgo(t *testing.T) {
	ts := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	result := agentic.FormatBriefTimestamp(ts, refTime)
	assert.Contains(t, result, "Tuesday")
	assert.Contains(t, result, "09:00")
}

func TestFormatBriefTimestamp_SixDaysAgo(t *testing.T) {
	ts := time.Date(2026, 3, 28, 12, 30, 0, 0, time.UTC)
	result := agentic.FormatBriefTimestamp(ts, refTime)
	assert.Contains(t, result, "Saturday")
	assert.Contains(t, result, "12:30")
}

func TestFormatBriefTimestamp_SevenDaysAgo(t *testing.T) {
	ts := time.Date(2026, 3, 27, 8, 0, 0, 0, time.UTC)
	result := agentic.FormatBriefTimestamp(ts, refTime)
	assert.Contains(t, result, "Friday")
	assert.Contains(t, result, "Mar")
	assert.Contains(t, result, "27")
	assert.Contains(t, result, "08:00")
}

func TestFormatBriefTimestamp_OlderDate(t *testing.T) {
	ts := time.Date(2026, 1, 15, 20, 0, 0, 0, time.UTC)
	result := agentic.FormatBriefTimestamp(ts, refTime)
	assert.Contains(t, result, "Thursday")
	assert.Contains(t, result, "Jan")
	assert.Contains(t, result, "15")
	assert.Contains(t, result, "20:00")
}

func TestFormatBriefTimestamp_Zero(t *testing.T) {
	result := agentic.FormatBriefTimestamp(time.Time{}, refTime)
	assert.Equal(t, "", result)
}

func TestFormatBriefTimestampFromISO_Valid(t *testing.T) {
	result := agentic.FormatBriefTimestampFromISO("2026-04-03T10:15:00Z", refTime)
	assert.Equal(t, "10:15", result)
}

func TestFormatBriefTimestampFromISO_Invalid(t *testing.T) {
	result := agentic.FormatBriefTimestampFromISO("not-a-date", refTime)
	assert.Equal(t, "", result)
}

func TestFormatBriefTimestampFromISO_NoTimezone(t *testing.T) {
	result := agentic.FormatBriefTimestampFromISO("2026-04-03T10:15:00", refTime)
	assert.Equal(t, "10:15", result)
}

// --- FormatRelativeTime ---

func TestFormatRelativeTime_JustNow(t *testing.T) {
	ts := refTime.Add(-30 * time.Second)
	assert.Equal(t, "just now", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_OneMinute(t *testing.T) {
	ts := refTime.Add(-1 * time.Minute)
	assert.Equal(t, "1 minute ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_Minutes(t *testing.T) {
	ts := refTime.Add(-15 * time.Minute)
	assert.Equal(t, "15 minutes ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_OneHour(t *testing.T) {
	ts := refTime.Add(-1 * time.Hour)
	assert.Equal(t, "1 hour ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_Hours(t *testing.T) {
	ts := refTime.Add(-5 * time.Hour)
	assert.Equal(t, "5 hours ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_Yesterday(t *testing.T) {
	ts := refTime.Add(-25 * time.Hour)
	assert.Equal(t, "yesterday", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_Days(t *testing.T) {
	ts := refTime.Add(-5 * 24 * time.Hour)
	assert.Equal(t, "5 days ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_OneMonth(t *testing.T) {
	ts := refTime.Add(-35 * 24 * time.Hour)
	assert.Equal(t, "1 month ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_Months(t *testing.T) {
	ts := refTime.Add(-90 * 24 * time.Hour)
	assert.Equal(t, "3 months ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_OneYear(t *testing.T) {
	ts := refTime.Add(-400 * 24 * time.Hour)
	assert.Equal(t, "1 year ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_Years(t *testing.T) {
	ts := refTime.Add(-800 * 24 * time.Hour)
	assert.Equal(t, "2 years ago", agentic.FormatRelativeTime(ts, refTime))
}

func TestFormatRelativeTime_Future(t *testing.T) {
	ts := refTime.Add(1 * time.Hour)
	assert.Equal(t, "in the future", agentic.FormatRelativeTime(ts, refTime))
}
