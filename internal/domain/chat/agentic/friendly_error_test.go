package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- TR-01-TASK-33: mensagens amigáveis para LLM indisponível vs timeout (P-C252-1) ---

func TestFriendlyRunErrorMessage_Timeout_DeadlineExceeded(t *testing.T) {
	msg := friendlyRunErrorMessage("stream_consume", "context deadline exceeded")
	assert.True(t, strings.Contains(msg, "timeout"), "should mention timeout")
}

func TestFriendlyRunErrorMessage_Timeout_ContextDeadline(t *testing.T) {
	msg := friendlyRunErrorMessage("llm_call", "context deadline exceeded while waiting for response")
	assert.True(t, strings.Contains(msg, "timeout"), "should mention timeout")
}

func TestFriendlyRunErrorMessage_Timeout_Keyword(t *testing.T) {
	msg := friendlyRunErrorMessage("llm_call", "request timeout after 900s")
	assert.True(t, strings.Contains(msg, "timeout"), "should mention timeout")
}

func TestFriendlyRunErrorMessage_Unavailable_503(t *testing.T) {
	msg := friendlyRunErrorMessage("stream_consume", "503 Service Unavailable")
	assert.True(t, strings.Contains(msg, "indisponível"), "should say unavailable")
	assert.False(t, strings.Contains(msg, "timeout"), "should NOT say timeout")
}

func TestFriendlyRunErrorMessage_Unavailable_Keyword(t *testing.T) {
	msg := friendlyRunErrorMessage("llm_call", "service unavailable")
	assert.True(t, strings.Contains(msg, "indisponível"), "should say unavailable")
	assert.False(t, strings.Contains(msg, "timeout"), "should NOT say timeout")
}

func TestFriendlyRunErrorMessage_RateLimit(t *testing.T) {
	msg := friendlyRunErrorMessage("llm_call", "429 rate limit exceeded")
	assert.True(t, strings.Contains(msg, "sobrecarregado") || strings.Contains(msg, "rate limit"), "should mention rate limit")
}

func TestFriendlyRunErrorMessage_Unauthorized(t *testing.T) {
	msg := friendlyRunErrorMessage("llm_call", "401 Unauthorized")
	assert.True(t, strings.Contains(msg, "credenciais"), "should mention credentials")
}

func TestFriendlyRunErrorMessage_ContextCancelled(t *testing.T) {
	msg := friendlyRunErrorMessage("context_cancelled", "")
	assert.True(t, strings.Contains(msg, "cancelada"), "should say cancelled")
}

func TestFriendlyRunErrorMessage_Timeout_TakesPriorityOver503(t *testing.T) {
	// An error that contains both "timeout" and "503" should be treated as timeout.
	msg := friendlyRunErrorMessage("stream_consume", "timeout reading 503 response body")
	assert.True(t, strings.Contains(msg, "timeout"), "timeout check should take priority")
}

func TestFriendlyRunErrorMessage_UnknownCode(t *testing.T) {
	msg := friendlyRunErrorMessage("some_other_code", "unknown error")
	assert.True(t, strings.Contains(msg, "some_other_code"), "should include error code")
}
