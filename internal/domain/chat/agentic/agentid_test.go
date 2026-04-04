package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- FormatAgentID ---

func TestFormatAgentID_Basic(t *testing.T) {
	assert.Equal(t, "researcher@my-project", agentic.FormatAgentID("researcher", "my-project"))
}

func TestFormatAgentID_WithHyphens(t *testing.T) {
	assert.Equal(t, "team-lead@my-project", agentic.FormatAgentID("team-lead", "my-project"))
}

// --- ParseAgentID ---

func TestParseAgentID_Valid(t *testing.T) {
	result := agentic.ParseAgentID("researcher@my-project")
	require.NotNil(t, result)
	assert.Equal(t, "researcher", result.AgentName)
	assert.Equal(t, "my-project", result.TeamName)
}

func TestParseAgentID_NoAt(t *testing.T) {
	assert.Nil(t, agentic.ParseAgentID("no-at-sign"))
}

func TestParseAgentID_MultipleAt(t *testing.T) {
	result := agentic.ParseAgentID("agent@team@extra")
	require.NotNil(t, result)
	assert.Equal(t, "agent", result.AgentName)
	assert.Equal(t, "team@extra", result.TeamName)
}

func TestParseAgentID_EmptyName(t *testing.T) {
	result := agentic.ParseAgentID("@team")
	require.NotNil(t, result)
	assert.Equal(t, "", result.AgentName)
	assert.Equal(t, "team", result.TeamName)
}

// --- GenerateRequestIDAt ---

func TestGenerateRequestIDAt_Format(t *testing.T) {
	id := agentic.GenerateRequestIDAt("shutdown", "researcher@my-project", 1702500000000)
	assert.Equal(t, "shutdown-1702500000000@researcher@my-project", id)
}

// --- ParseRequestID ---

func TestParseRequestID_Valid(t *testing.T) {
	result := agentic.ParseRequestID("shutdown-1702500000000@researcher@my-project")
	require.NotNil(t, result)
	assert.Equal(t, "shutdown", result.RequestType)
	assert.Equal(t, int64(1702500000000), result.Timestamp)
	assert.Equal(t, "researcher@my-project", result.AgentID)
}

func TestParseRequestID_NoAt(t *testing.T) {
	assert.Nil(t, agentic.ParseRequestID("no-at-sign"))
}

func TestParseRequestID_NoDash(t *testing.T) {
	assert.Nil(t, agentic.ParseRequestID("nodash@agent"))
}

func TestParseRequestID_InvalidTimestamp(t *testing.T) {
	assert.Nil(t, agentic.ParseRequestID("shutdown-abc@agent@team"))
}

func TestParseRequestID_CompoundType(t *testing.T) {
	result := agentic.ParseRequestID("plan-approval-1702500000000@agent@team")
	require.NotNil(t, result)
	assert.Equal(t, "plan-approval", result.RequestType)
	assert.Equal(t, int64(1702500000000), result.Timestamp)
}

// --- Roundtrip ---

func TestAgentID_Roundtrip(t *testing.T) {
	original := agentic.FormatAgentID("tester", "my-team")
	parsed := agentic.ParseAgentID(original)
	require.NotNil(t, parsed)
	assert.Equal(t, "tester", parsed.AgentName)
	assert.Equal(t, "my-team", parsed.TeamName)
}

func TestRequestID_Roundtrip(t *testing.T) {
	agentID := agentic.FormatAgentID("agent", "team")
	reqID := agentic.GenerateRequestIDAt("shutdown", agentID, 12345)
	parsed := agentic.ParseRequestID(reqID)
	require.NotNil(t, parsed)
	assert.Equal(t, "shutdown", parsed.RequestType)
	assert.Equal(t, int64(12345), parsed.Timestamp)
	assert.Equal(t, agentID, parsed.AgentID)
}

// --- SanitizeAgentName ---

func TestSanitizeAgentName_NoAt(t *testing.T) {
	assert.Equal(t, "researcher", agentic.SanitizeAgentName("researcher"))
}

func TestSanitizeAgentName_WithAt(t *testing.T) {
	assert.Equal(t, "user123", agentic.SanitizeAgentName("user@123"))
}

func TestSanitizeAgentName_MultipleAt(t *testing.T) {
	assert.Equal(t, "abc", agentic.SanitizeAgentName("a@b@c"))
}
