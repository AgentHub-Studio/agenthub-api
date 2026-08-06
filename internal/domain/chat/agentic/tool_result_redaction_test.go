package agentic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSensitiveToolResultOutput_RemovesNestedSensitiveKeys(t *testing.T) {
	raw := json.RawMessage(`{
		"ok": true,
		"auth_token": "short-token",
		"nested": {
			"apiKey": "api-key",
			"password": "pw",
			"safe": "kept"
		},
		"items": [
			{"secret": "secret-value", "name": "kept-name"}
		]
	}`)

	got := redactSensitiveToolResultOutput(raw)

	assert.NotContains(t, string(got), "auth_token")
	assert.NotContains(t, string(got), "short-token")
	assert.NotContains(t, string(got), "apiKey")
	assert.NotContains(t, string(got), "api-key")
	assert.NotContains(t, string(got), "password")
	assert.NotContains(t, string(got), "secret-value")
	assert.Contains(t, string(got), "kept")
	assert.Contains(t, string(got), "kept-name")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(got, &decoded))
	require.NotContains(t, decoded, "auth_token")
}

func TestRedactSensitiveToolResultOutput_RemovesAuthorizationHeaders(t *testing.T) {
	raw := json.RawMessage(`{
		"ok": true,
		"headers": {
			"Authorization": "Bearer abcdefghijklmnopqrstuvwx.yyyyyyyyyyyyyyyyyy.zzzzzzzzzzzzzzzzzz",
			"Proxy-Authorization": "Basic dXNlcjp2ZXJ5LXNlY3JldA==",
			"request_id": "req-123"
		}
	}`)

	got := redactSensitiveToolResultOutput(raw)
	gotString := string(got)

	assert.NotContains(t, gotString, "Authorization")
	assert.NotContains(t, gotString, "Proxy-Authorization")
	assert.NotContains(t, gotString, "Bearer abcdefghijklmnopqrstuvwx")
	assert.NotContains(t, gotString, "dXNlcjp2ZXJ5LXNlY3JldA")
	assert.Contains(t, gotString, "req-123")
	require.True(t, json.Valid(got))
}

func TestRedactSensitiveToolResultOutput_RemovesCookieAndPlainHeaderSecrets(t *testing.T) {
	structured := json.RawMessage(`{
		"headers": {
			"Cookie": "session=very-sensitive-session-value",
			"Set-Cookie": "session=very-sensitive-session-value; HttpOnly",
			"request_id": "req-123"
		}
	}`)

	gotStructured := redactSensitiveToolResultOutput(structured)
	assert.NotContains(t, string(gotStructured), "very-sensitive-session-value")
	assert.Contains(t, string(gotStructured), "req-123")
	require.True(t, json.Valid(gotStructured))

	plain := json.RawMessage("Authorization: Basic dXNlcjphLWZha2Utc2VjcmV0\nCookie: session=very-sensitive-session-value\nrequest_id=req-456")
	gotPlain := redactSensitiveToolResultOutput(plain)
	assert.NotContains(t, string(gotPlain), "dXNlcjphLWZha2Utc2VjcmV0")
	assert.NotContains(t, string(gotPlain), "very-sensitive-session-value")
	assert.Contains(t, string(gotPlain), "request_id=req-456")
}

func TestSanitizeToolErrorRedactsSecretsAndInternalTopology(t *testing.T) {
	const secret = "tool-error-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"

	got := sanitizeToolError("Authorization: Bearer " + secret + "\nupstream=" + internalURL)

	assert.NotContains(t, got, secret)
	assert.NotContains(t, got, internalURL)
	assert.Contains(t, got, "[REDACTED]")
	assert.Contains(t, got, "<upstream>")
}

func TestSummarizeResultRedactsSecretsAndInternalTopology(t *testing.T) {
	const secret = "subtask-summary-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	message := "Authorization: Bearer " + secret + "\nupstream=" + internalURL

	for _, tc := range []struct {
		name string
		got  string
	}{
		{name: "content", got: summarizeResult(message, nil)},
		{name: "error", got: summarizeResult("", &message)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotContains(t, tc.got, secret)
			assert.NotContains(t, tc.got, internalURL)
			assert.Contains(t, tc.got, "[REDACTED]")
			assert.Contains(t, tc.got, "<upstream>")
		})
	}
}

func TestStreamingToolExecutor_EmitsSerializableRedactedPlainHeaderResult(t *testing.T) {
	executor := NewStreamingToolExecutor(nil, nil, RunConfig{})
	tracked := newTrackedTool("call-1", "http_request", nil)
	events := make(chan RunEvent, 1)

	executor.emitToolResult(events, tracked, ToolExecResult{
		Output: json.RawMessage("Authorization: Basic dXNlcjphLWZha2Utc2VjcmV0\nCookie: session=very-sensitive-session-value\nrequest_id=req-456"),
	})

	event := <-events
	require.Equal(t, EventToolResult, event.Type)
	require.True(t, json.Valid(event.Data), "SSE event must be serializable JSON")

	var data ToolResultData
	require.NoError(t, json.Unmarshal(event.Data, &data))
	require.True(t, json.Valid(data.Output), "SSE output must be valid JSON")
	assert.NotContains(t, string(data.Output), "dXNlcjphLWZha2Utc2VjcmV0")
	assert.NotContains(t, string(data.Output), "very-sensitive-session-value")
	assert.Contains(t, string(data.Output), "request_id=req-456")
}

func FuzzRedactSensitiveToolResultOutputRemovesSensitiveData(f *testing.F) {
	f.Add("Authorization", "Bearer abcdefghijklmnopqrstuvwx.yyyyyyyyyyyyyyyyyy.zzzzzzzzzzzzzzzzzz", "safe")
	f.Add("Proxy-Authorization", "Basic dXNlcjp2ZXJ5LXNlY3JldA==", "safe")
	f.Add("api_key", "short-api-key", "safe")
	f.Add("clientSecret", "short-client-secret", "safe")
	f.Add("password", "short-password", "safe")
	f.Add("secret", "short-secret", "safe")

	f.Fuzz(func(t *testing.T, keySeed, secretSeed, safeSeed string) {
		key := redactionSensitiveKeyVariant(keySeed)
		secret := redactionSecretValue(secretSeed)
		safeValue := "safe-marker-" + redactionSafeSuffix(safeSeed)
		raw, err := json.Marshal(map[string]any{
			"headers": map[string]any{
				key:          secret,
				"request_id": safeValue,
			},
			"items": []any{
				map[string]any{
					key:    secret,
					"name": safeValue,
				},
			},
			"safe_message": "scanner should redact " + redactionStandaloneScannerSecret,
		})
		require.NoError(t, err)

		got := redactSensitiveToolResultOutput(raw)
		gotString := string(got)
		if !json.Valid(got) {
			t.Fatalf("redacted output is not valid JSON: %q", gotString)
		}
		if strings.Contains(gotString, secret) {
			t.Fatalf("sensitive value leaked for key %q: %q", key, gotString)
		}
		if strings.Contains(gotString, redactionStandaloneScannerSecret) {
			t.Fatalf("standalone scanner secret leaked: %q", gotString)
		}
		if HasSecrets(gotString) {
			t.Fatalf("redacted output still matches secret scanner: %q", gotString)
		}
		if !strings.Contains(gotString, safeValue) {
			t.Fatalf("safe value was removed unexpectedly: %q", gotString)
		}
	})
}

func FuzzRedactSensitiveToolResultOutputRemovesPlainHeaderSecrets(f *testing.F) {
	f.Add("Authorization", "header-secret", "safe")
	f.Add("Proxy-Authorization", "proxy-secret", "safe")
	f.Add("Cookie", "session-secret", "safe")
	f.Add("Set-Cookie", "set-cookie-secret", "safe")
	f.Add("X-API-Key", "api-secret", "safe")

	f.Fuzz(func(t *testing.T, headerSeed, secretSeed, safeSeed string) {
		header := redactionPlainHeaderName(headerSeed)
		secret := "secret-" + redactionSafeSuffix(secretSeed)
		safeValue := "safe-marker-" + redactionSafeSuffix(safeSeed)
		raw := json.RawMessage(header + ": " + secret + "\nrequest_id=" + safeValue)

		got := redactSensitiveToolResultOutput(raw)
		gotString := string(got)
		if strings.Contains(gotString, secret) {
			t.Fatalf("plain sensitive header leaked for %q: %q", header, gotString)
		}
		if !strings.Contains(gotString, safeValue) {
			t.Fatalf("safe plain value was removed unexpectedly: %q", gotString)
		}
		if !json.Valid(serializableJSONRawMessage(got)) {
			t.Fatalf("plain redacted output is not serializable for SSE: %q", gotString)
		}
	})
}

func FuzzNewRunEventRedactsToolCallInput(f *testing.F) {
	f.Add("Authorization", "header-secret", "safe")
	f.Add("Proxy-Authorization", "proxy-secret", "safe")
	f.Add("Cookie", "session-secret", "safe")
	f.Add("Set-Cookie", "set-cookie-secret", "safe")
	f.Add("X-API-Key", "api-secret", "safe")

	f.Fuzz(func(t *testing.T, headerSeed, secretSeed, safeSeed string) {
		header := redactionPlainHeaderName(headerSeed)
		secret := "secret-" + redactionSafeSuffix(secretSeed)
		safeValue := "safe-marker-" + redactionSafeSuffix(safeSeed)
		raw, err := json.Marshal(map[string]any{
			"headers": map[string]string{header: secret},
			"request": safeValue,
		})
		if err != nil {
			t.Fatalf("marshal test input: %v", err)
		}

		event := NewRunEvent(EventToolCallStart, ToolCallStartData{Input: raw})
		if strings.Contains(string(event.Data), secret) {
			t.Fatalf("tool_call_start leaked %q through %q: %q", secret, header, event.Data)
		}
		if !strings.Contains(string(event.Data), safeValue) {
			t.Fatalf("tool_call_start removed safe input: %q", event.Data)
		}
		if !json.Valid(event.Data) {
			t.Fatalf("tool_call_start envelope is not valid JSON: %q", event.Data)
		}
	})
}

func FuzzNewRunEventRedactsDiagnosticPayloads(f *testing.F) {
	f.Add("error", "header-secret")
	f.Add("fallback", "fallback-secret")
	f.Add("hook", "hook-secret")
	f.Add("subtask", "subtask-secret")

	f.Fuzz(func(t *testing.T, kindSeed, secretSeed string) {
		secret := "diagnostic-" + redactionSafeSuffix(secretSeed)
		message := "Authorization: Bearer " + secret + "\nprovider=http://agenthub-provider:8080/v1/chat"
		event, field := diagnosticRedactionEvent(kindSeed, message)

		var data map[string]json.RawMessage
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode diagnostic event: %v", err)
		}
		var got string
		if err := json.Unmarshal(data[field], &got); err != nil {
			t.Fatalf("decode diagnostic field: %v", err)
		}
		if strings.Contains(got, secret) {
			t.Fatalf("diagnostic event leaked secret for %q: %q", kindSeed, got)
		}
		if strings.Contains(got, "http://agenthub-provider:8080/v1/chat") {
			t.Fatalf("diagnostic event leaked internal topology for %q: %q", kindSeed, got)
		}
	})
}

func FuzzNewRunEventRedactsToolResultError(f *testing.F) {
	f.Add("Authorization", "tool-error-secret")
	f.Add("Cookie", "session-secret")
	f.Add("X-API-Key", "api-secret")

	f.Fuzz(func(t *testing.T, headerSeed, secretSeed string) {
		header := redactionPlainHeaderName(headerSeed)
		secret := "tool-error-" + redactionSafeSuffix(secretSeed)
		message := header + ": Bearer " + secret + "\nupstream=http://agenthub-skill-runtime:8080/v1/execute"
		event := NewRunEvent(EventToolResult, ToolResultData{Error: &message})

		var data ToolResultData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode tool result event: %v", err)
		}
		if data.Error == nil {
			t.Fatal("tool result error unexpectedly omitted")
		}
		if !data.IsError {
			t.Fatal("tool result error flag unexpectedly false")
		}
		if strings.Contains(*data.Error, secret) {
			t.Fatalf("tool result error leaked secret through %q: %q", header, *data.Error)
		}
		if strings.Contains(*data.Error, "http://agenthub-skill-runtime:8080/v1/execute") {
			t.Fatalf("tool result error leaked internal topology: %q", *data.Error)
		}
	})
}

func FuzzNewRunEventDerivesToolResultErrorFlag(f *testing.F) {
	f.Add(false, "success")
	f.Add(true, "tool failure")

	f.Fuzz(func(t *testing.T, shouldFail bool, message string) {
		payload := ToolResultData{Output: json.RawMessage(`{"ok":true}`)}
		if shouldFail {
			payload.Error = &message
		}

		event := NewRunEvent(EventToolResult, payload)
		var data ToolResultData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode tool result event: %v", err)
		}
		if data.IsError != shouldFail {
			t.Fatalf("tool result isError = %t, want %t", data.IsError, shouldFail)
		}
	})
}

func FuzzNewRunEventRedactsSubtaskSummaries(f *testing.F) {
	f.Add("progress", "summary-secret")
	f.Add("completion", "completion-secret")

	f.Fuzz(func(t *testing.T, kindSeed, secretSeed string) {
		secret := "subtask-" + redactionSafeSuffix(secretSeed)
		summary := "Authorization: Bearer " + secret + "\nupstream=http://agenthub-skill-runtime:8080/v1/execute"
		var event RunEvent
		if len(kindSeed)%2 == 0 {
			event = NewRunEvent(EventSubtaskProgress, SubtaskProgressData{Summary: summary})
		} else {
			event = NewRunEvent(EventSubtaskComplete, SubtaskCompleteData{Summary: summary})
		}

		var data map[string]json.RawMessage
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode subtask summary event: %v", err)
		}
		var got string
		if err := json.Unmarshal(data["summary"], &got); err != nil {
			t.Fatalf("decode subtask summary: %v", err)
		}
		if strings.Contains(got, secret) {
			t.Fatalf("subtask summary leaked secret: %q", got)
		}
		if strings.Contains(got, "http://agenthub-skill-runtime:8080/v1/execute") {
			t.Fatalf("subtask summary leaked internal topology: %q", got)
		}
	})
}

func FuzzNewRunEventRedactsAgentMessage(f *testing.F) {
	f.Add("mailbox-secret")

	f.Fuzz(func(t *testing.T, secretSeed string) {
		secret := "agent-message-" + redactionSafeSuffix(secretSeed)
		content := "Authorization: Bearer " + secret + "\nupstream=http://agenthub-skill-runtime:8080/v1/execute"
		event := NewRunEvent(EventAgentMessage, AgentMessageData{
			ID: "message-1", From: "subtask-a", To: "subtask-b", Content: content,
		})

		var data AgentMessageData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode agent message event: %v", err)
		}
		if strings.Contains(data.Content, secret) {
			t.Fatalf("agent message leaked secret: %q", data.Content)
		}
		if strings.Contains(data.Content, "http://agenthub-skill-runtime:8080/v1/execute") {
			t.Fatalf("agent message leaked internal topology: %q", data.Content)
		}
	})
}

func FuzzNewRunEventRedactsSubtaskStart(f *testing.F) {
	f.Add("delegation-secret")

	f.Fuzz(func(t *testing.T, secretSeed string) {
		secret := "subtask-start-" + redactionSafeSuffix(secretSeed)
		description := "Authorization: Bearer " + secret + "\nupstream=http://agenthub-skill-runtime:8080/v1/execute"
		event := NewRunEvent(EventSubtaskStart, SubtaskStartData{
			ID: "subtask-1", Description: description, Depth: 1,
		})

		var data SubtaskStartData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode subtask start event: %v", err)
		}
		if strings.Contains(data.Description, secret) {
			t.Fatalf("subtask start leaked secret: %q", data.Description)
		}
		if strings.Contains(data.Description, "http://agenthub-skill-runtime:8080/v1/execute") {
			t.Fatalf("subtask start leaked internal topology: %q", data.Description)
		}
	})
}

func FuzzNewRunEventRedactsRunProgressActivity(f *testing.F) {
	f.Add("progress-secret")

	f.Fuzz(func(t *testing.T, secretSeed string) {
		secret := "run-progress-" + redactionSafeSuffix(secretSeed)
		summary := "Started: Authorization: Bearer " + secret + "\nupstream=http://agenthub-skill-runtime:8080/v1/execute"
		event := NewRunEvent(EventRunProgress, RunProgressData{
			RecentActivity: []ActivityItem{{Type: ActivitySubtask, Summary: summary}},
		})

		var data RunProgressData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode run progress event: %v", err)
		}
		if len(data.RecentActivity) != 1 {
			t.Fatalf("run progress activity count = %d, want 1", len(data.RecentActivity))
		}
		if strings.Contains(data.RecentActivity[0].Summary, secret) {
			t.Fatalf("run progress leaked secret: %q", data.RecentActivity[0].Summary)
		}
		if strings.Contains(data.RecentActivity[0].Summary, "http://agenthub-skill-runtime:8080/v1/execute") {
			t.Fatalf("run progress leaked internal topology: %q", data.RecentActivity[0].Summary)
		}
	})
}

func diagnosticRedactionEvent(seed, message string) (RunEvent, string) {
	switch seed {
	case "error":
		return NewRunEvent(EventError, ErrorData{Message: message}), "message"
	case "warning":
		return NewRunEvent(EventWarning, WarningData{Message: message}), "message"
	case "fallback":
		return NewRunEvent(EventModelFallback, ModelFallbackData{Reason: message}), "reason"
	case "denied":
		return NewRunEvent(EventToolDenied, ToolDeniedData{Reason: message}), "reason"
	case "hook":
		return NewRunEvent(EventStopHookSummary, StopHookSummaryData{Summary: message}), "summary"
	case "subtask":
		return NewRunEvent(EventSubtaskComplete, SubtaskCompleteData{Error: &message}), "error"
	}

	sum := 0
	for i := 0; i < len(seed); i++ {
		sum += int(seed[i])
	}
	return diagnosticRedactionEvent([]string{"error", "warning", "fallback", "denied", "hook", "subtask"}[sum%6], message)
}

const redactionStandaloneScannerSecret = "sk-ant-abcdefghijklmnopqrstuvwxyz123456"

var redactionSensitiveKeyVariants = []string{
	"auth_token",
	"auth-token",
	"auth.token",
	"Authorization",
	"Proxy-Authorization",
	"Cookie",
	"Set-Cookie",
	"X-Auth-Token",
	"apiKey",
	"api_key",
	"x-api-key",
	"password",
	"secret",
	"clientSecret",
	"bearerToken",
	"access_token",
	"refresh_token",
}

var redactionPlainHeaderNames = []string{
	"Authorization",
	"Proxy-Authorization",
	"Cookie",
	"Set-Cookie",
	"X-API-Key",
	"X-Auth-Token",
}

func redactionPlainHeaderName(seed string) string {
	for _, header := range redactionPlainHeaderNames {
		if seed == header {
			return header
		}
	}
	sum := 0
	for i := 0; i < len(seed); i++ {
		sum += int(seed[i])
	}
	return redactionPlainHeaderNames[sum%len(redactionPlainHeaderNames)]
}

func redactionSensitiveKeyVariant(seed string) string {
	for _, key := range redactionSensitiveKeyVariants {
		if seed == key {
			return key
		}
	}
	sum := 0
	for i := 0; i < len(seed); i++ {
		sum += int(seed[i])
	}
	return redactionSensitiveKeyVariants[sum%len(redactionSensitiveKeyVariants)]
}

func redactionSecretValue(seed string) string {
	if strings.HasPrefix(seed, "Bearer ") || strings.HasPrefix(seed, "Basic ") {
		return seed
	}
	sum := 0
	for i := 0; i < len(seed); i++ {
		sum += int(seed[i])
	}
	if sum%2 == 0 {
		return "secret-value"
	}
	return redactionStandaloneScannerSecret
}

func redactionSafeSuffix(seed string) string {
	var b strings.Builder
	for i := 0; i < len(seed) && b.Len() < 32; i++ {
		ch := seed[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteByte(ch)
		case ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		}
	}
	if b.Len() == 0 {
		return "value"
	}
	return b.String()
}
