package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- FormatFileSize ---

func TestFormatFileSize_Bytes(t *testing.T) {
	assert.Equal(t, "512 bytes", agentic.FormatFileSize(512))
}

func TestFormatFileSize_KB(t *testing.T) {
	assert.Equal(t, "1.5KB", agentic.FormatFileSize(1536))
}

func TestFormatFileSize_MB(t *testing.T) {
	assert.Equal(t, "2MB", agentic.FormatFileSize(2*1024*1024))
}

func TestFormatFileSize_GB(t *testing.T) {
	assert.Equal(t, "1.5GB", agentic.FormatFileSize(int64(1.5*1024*1024*1024)))
}

func TestFormatFileSize_Zero(t *testing.T) {
	assert.Equal(t, "0 bytes", agentic.FormatFileSize(0))
}

func TestFormatFileSize_ExactKB(t *testing.T) {
	assert.Equal(t, "1KB", agentic.FormatFileSize(1024))
}

// --- FormatDuration ---

func TestFormatDuration_Zero(t *testing.T) {
	assert.Equal(t, "0s", agentic.FormatDuration(0))
}

func TestFormatDuration_SubSecond(t *testing.T) {
	assert.Equal(t, "0.5s", agentic.FormatDuration(500))
}

func TestFormatDuration_Seconds(t *testing.T) {
	assert.Equal(t, "5s", agentic.FormatDuration(5000))
}

func TestFormatDuration_MinutesSeconds(t *testing.T) {
	assert.Equal(t, "1m 5s", agentic.FormatDuration(65000))
}

func TestFormatDuration_Hours(t *testing.T) {
	assert.Equal(t, "1h 1m 1s", agentic.FormatDuration(3661000))
}

func TestFormatDuration_Days(t *testing.T) {
	assert.Equal(t, "1d 2h 30m", agentic.FormatDuration(95400000))
}

// --- FormatDurationCompact ---

func TestFormatDurationCompact_Seconds(t *testing.T) {
	assert.Equal(t, "5s", agentic.FormatDurationCompact(5000))
}

func TestFormatDurationCompact_Minutes(t *testing.T) {
	assert.Equal(t, "2m", agentic.FormatDurationCompact(120000))
}

func TestFormatDurationCompact_Hours(t *testing.T) {
	assert.Equal(t, "3h", agentic.FormatDurationCompact(10800000))
}

func TestFormatDurationCompact_Days(t *testing.T) {
	assert.Equal(t, "2d", agentic.FormatDurationCompact(172800000))
}

// --- FormatNumber ---

func TestFormatNumber_Small(t *testing.T) {
	assert.Equal(t, "900", agentic.FormatNumber(900))
}

func TestFormatNumber_Thousands(t *testing.T) {
	assert.Equal(t, "1.3k", agentic.FormatNumber(1321))
}

func TestFormatNumber_Millions(t *testing.T) {
	assert.Equal(t, "1.5m", agentic.FormatNumber(1500000))
}

func TestFormatNumber_Billions(t *testing.T) {
	assert.Equal(t, "2.0b", agentic.FormatNumber(2000000000))
}

func TestFormatNumber_Zero(t *testing.T) {
	assert.Equal(t, "0", agentic.FormatNumber(0))
}

func TestFormatNumber_ExactThousand(t *testing.T) {
	assert.Equal(t, "1.0k", agentic.FormatNumber(1000))
}

// --- FormatTokens ---

func TestFormatTokens_Small(t *testing.T) {
	assert.Equal(t, "500", agentic.FormatTokens(500))
}

func TestFormatTokens_Exact(t *testing.T) {
	assert.Equal(t, "1k", agentic.FormatTokens(1000))
}

func TestFormatTokens_WithDecimal(t *testing.T) {
	assert.Equal(t, "1.5k", agentic.FormatTokens(1500))
}

func TestFormatTokens_Millions(t *testing.T) {
	assert.Equal(t, "1m", agentic.FormatTokens(1000000))
}
