package chat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func FuzzDecodeJSONRequestRejectsTrailingValue(f *testing.F) {
	for _, suffix := range [][]byte{
		[]byte(`{"message":"ignored"}`),
		[]byte(`null`),
		[]byte(`malformed`),
		[]byte("\n\t"),
	} {
		f.Add(suffix)
	}

	f.Fuzz(func(t *testing.T, suffix []byte) {
		if len(bytes.TrimSpace(suffix)) == 0 {
			return
		}

		body := append([]byte(`{"message":"first"}`), suffix...)
		req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/run", bytes.NewReader(body))
		var target struct {
			Message string `json:"message"`
		}
		if err := decodeJSONRequest(req, &target); err == nil {
			t.Fatal("expected a trailing value to be rejected")
		}
	})
}

func FuzzRunResponseFromRedactsSensitiveMetadata(f *testing.F) {
	f.Add("error-secret", "safe-value")
	f.Add("credential-token", "request-123")

	f.Fuzz(func(t *testing.T, secretSeed, safeSeed string) {
		secretSum := sha256.Sum256([]byte(secretSeed))
		safeSum := sha256.Sum256([]byte(safeSeed))
		secret := "run-metadata-secret-" + hex.EncodeToString(secretSum[:])
		safeValue := "safe-marker-" + hex.EncodeToString(safeSum[:])
		metadata, err := json.Marshal(map[string]any{
			"errorMessage": "Authorization: Bearer " + secret + "\npassword=" + secret,
			"nested": map[string]string{
				"api_key": secret,
				"request": safeValue,
			},
		})
		if err != nil {
			t.Fatalf("marshal metadata: %v", err)
		}

		run := ChatRun{ID: uuid.New(), Metadata: metadata}
		response := RunResponseFrom(run)
		public := string(response.Metadata)
		if !json.Valid(response.Metadata) {
			t.Fatalf("redacted metadata is not valid JSON: %q", public)
		}
		if strings.Contains(public, secret) {
			t.Fatalf("public metadata leaked secret: %q", public)
		}
		if strings.Contains(public, "api_key") {
			t.Fatalf("public metadata retained sensitive key: %q", public)
		}
		if !strings.Contains(public, safeValue) {
			t.Fatalf("public metadata removed safe value: %q", public)
		}
		if !strings.Contains(public, "[REDACTED]") {
			t.Fatalf("public metadata lacks redaction marker: %q", public)
		}
		if string(run.Metadata) != string(metadata) {
			t.Fatalf("RunResponseFrom mutated persisted metadata")
		}
	})
}

func FuzzMessageResponseFromRedactsSensitiveToolCallArguments(f *testing.F) {
	f.Add("tool-secret", "safe-input")
	f.Add("credential-token", "request-456")

	f.Fuzz(func(t *testing.T, secretSeed, safeSeed string) {
		secretSum := sha256.Sum256([]byte(secretSeed))
		safeSum := sha256.Sum256([]byte(safeSeed))
		secret := "tool-call-secret-" + hex.EncodeToString(secretSum[:])
		safeValue := "safe-input-" + hex.EncodeToString(safeSum[:])
		arguments, err := json.Marshal(map[string]any{
			"safe":          safeValue,
			"Authorization": "Bearer " + secret,
			"password":      secret,
			"nested": map[string]string{
				"api_key": secret,
			},
		})
		if err != nil {
			t.Fatalf("marshal tool arguments: %v", err)
		}
		toolCalls, err := json.Marshal([]map[string]any{{
			"id":   "tool-call-fuzz-1",
			"type": "function",
			"function": map[string]string{
				"name":      "http_request",
				"arguments": string(arguments),
			},
		}})
		if err != nil {
			t.Fatalf("marshal tool calls: %v", err)
		}

		message := ChatMessage{ID: uuid.New(), ToolCalls: toolCalls}
		response := MessageResponseFrom(message)
		public := string(response.ToolCalls)
		if !json.Valid(response.ToolCalls) {
			t.Fatalf("redacted tool calls are not valid JSON: %q", public)
		}
		if strings.Contains(public, secret) {
			t.Fatalf("public tool calls leaked secret: %q", public)
		}
		for _, sensitiveKey := range []string{"Authorization", "password", "api_key"} {
			if strings.Contains(public, sensitiveKey) {
				t.Fatalf("public tool calls retained sensitive key %q: %q", sensitiveKey, public)
			}
		}
		if !strings.Contains(public, safeValue) {
			t.Fatalf("public tool calls removed safe input: %q", public)
		}
		if string(message.ToolCalls) != string(toolCalls) {
			t.Fatalf("MessageResponseFrom mutated persisted tool calls")
		}
	})
}
