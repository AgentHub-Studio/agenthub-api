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

func TestSanitizeSSEMessageRedactsSecretsAndInternalTopology(t *testing.T) {
	message := "Authorization: Basic dXNlcjphLWZha2Utc2VjcmV0\nCookie: session=very-sensitive-session-value\nprovider=http://agenthub-provider:8080/v1/chat\nkey=sk-ant-abcdefghijklmnopqrstuvwxyz123456"

	got := sanitizeSSEMessage(message)

	assert.NotContains(t, got, "dXNlcjphLWZha2Utc2VjcmV0")
	assert.NotContains(t, got, "very-sensitive-session-value")
	assert.NotContains(t, got, "http://agenthub-provider:8080/v1/chat")
	assert.NotContains(t, got, "sk-ant-abcdefghijklmnopqrstuvwxyz123456")
	assert.Contains(t, got, "[REDACTED]")
	assert.Contains(t, got, "<upstream>")
}

func TestSanitizeSSEMessageRedactsEmbeddedHeaderSecrets(t *testing.T) {
	message := "Started: Authorization: Bearer embedded-header-secret\nupstream=http://agenthub-provider:8080/v1/chat"

	got := sanitizeSSEMessage(message)

	assert.NotContains(t, got, "embedded-header-secret")
	assert.NotContains(t, got, "http://agenthub-provider:8080/v1/chat")
	assert.Contains(t, got, "Started: [REDACTED]")
	assert.Contains(t, got, "<upstream>")
}

func FuzzSanitizeSSEMessageRedactsHeaders(f *testing.F) {
	f.Add("Authorization", "header-secret")
	f.Add("Proxy-Authorization", "proxy-secret")
	f.Add("Cookie", "session-secret")
	f.Add("Set-Cookie", "set-cookie-secret")
	f.Add("X-API-Key", "api-secret")

	f.Fuzz(func(t *testing.T, headerSeed, secretSeed string) {
		header := redactionPlainHeaderName(headerSeed)
		secret := "secret-" + redactionSafeSuffix(secretSeed)
		message := "Started: " + header + ": " + secret + "\nupstream=http://agenthub-provider:8080/v1/chat"

		got := sanitizeSSEMessage(message)
		if strings.Contains(got, secret) {
			t.Fatalf("SSE error leaked %q through %q: %q", secret, header, got)
		}
		if strings.Contains(got, "http://agenthub-provider:8080/v1/chat") {
			t.Fatalf("SSE error leaked internal URL: %q", got)
		}
		if !strings.Contains(got, "<upstream>") {
			t.Fatalf("SSE error removed safe topology marker: %q", got)
		}
	})
}
