package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseFrom_RedactsSensitiveAuditRecordValues(t *testing.T) {
	const apiKeySecret = "audit-old-api-key-sentinel"
	const authorizationSecret = "Bearer audit-new-authorization-sentinel"
	const refreshTokenSecret = "audit-metadata-refresh-token-sentinel"
	const clientCredentialSecret = "audit-client-credential-sentinel"
	const diagnosticSecret = "Bearer audit-metadata-authorization-sentinel"
	log := AuditLog{
		OldValue: `{"before":true,"apiKey":"audit-old-api-key-sentinel"}`,
		NewValue: `{"after":true,"headers":{"Authorization":"Bearer audit-new-authorization-sentinel"},"clientCredential":"audit-client-credential-sentinel"}`,
		Metadata: `{"operation":"update","diagnostic":"Authorization: Bearer audit-metadata-authorization-sentinel","nested":{"refresh_token":"audit-metadata-refresh-token-sentinel","source":"admin"}}`,
	}

	response := ResponseFrom(log)
	data, err := json.Marshal(response)
	require.NoError(t, err)
	body := string(data)

	assert.NotContains(t, body, apiKeySecret)
	assert.NotContains(t, body, authorizationSecret)
	assert.NotContains(t, body, refreshTokenSecret)
	assert.NotContains(t, body, clientCredentialSecret)
	assert.NotContains(t, body, diagnosticSecret)
	assert.NotContains(t, body, "apiKey")
	assert.NotContains(t, body, "Authorization")
	assert.NotContains(t, body, "refresh_token")
	assert.NotContains(t, body, "clientCredential")
	assert.Contains(t, response.OldValue, "before")
	assert.Contains(t, response.NewValue, "after")
	assert.Contains(t, response.Metadata, "admin")
	assert.JSONEq(t, `{"before":true,"apiKey":"audit-old-api-key-sentinel"}`, log.OldValue)
	assert.JSONEq(t, `{"after":true,"headers":{"Authorization":"Bearer audit-new-authorization-sentinel"},"clientCredential":"audit-client-credential-sentinel"}`, log.NewValue)
	assert.JSONEq(t, `{"operation":"update","diagnostic":"Authorization: Bearer audit-metadata-authorization-sentinel","nested":{"refresh_token":"audit-metadata-refresh-token-sentinel","source":"admin"}}`, log.Metadata)
}

func FuzzResponseFromRedactsSensitiveAuditRecordValues(f *testing.F) {
	f.Add("audit-secret-seed", "safe-audit-source")
	f.Add("credential-seed", "safe-audit-operation")

	f.Fuzz(func(t *testing.T, secretSeed, safeSeed string) {
		secretSum := sha256.Sum256([]byte(secretSeed))
		safeSum := sha256.Sum256([]byte(safeSeed))
		secret := "audit-secret-" + hex.EncodeToString(secretSum[:])
		safeValue := "safe-audit-value-" + hex.EncodeToString(safeSum[:])
		oldValue, err := json.Marshal(map[string]any{"api_key": secret, "before": safeValue})
		require.NoError(t, err)
		newValue, err := json.Marshal(map[string]any{"headers": map[string]string{"Authorization": "Bearer " + secret}, "after": safeValue})
		require.NoError(t, err)
		metadata, err := json.Marshal(map[string]any{"diagnostic": "Authorization: Bearer " + secret, "nested": map[string]string{"refreshToken": secret, "clientCredential": secret}, "source": safeValue})
		require.NoError(t, err)
		log := AuditLog{OldValue: string(oldValue), NewValue: string(newValue), Metadata: string(metadata)}

		response := ResponseFrom(log)
		public, err := json.Marshal(response)
		require.NoError(t, err)
		body := string(public)
		if strings.Contains(body, secret) {
			t.Fatalf("public audit response leaked secret: %q", body)
		}
		for _, sensitiveKey := range []string{"api_key", "Authorization", "refreshToken", "clientCredential"} {
			if strings.Contains(body, sensitiveKey) {
				t.Fatalf("public audit response retained sensitive key %q: %q", sensitiveKey, body)
			}
		}
		for _, field := range []string{response.OldValue, response.NewValue, response.Metadata} {
			if !json.Valid([]byte(field)) {
				t.Fatalf("redacted audit field is not valid JSON: %q", field)
			}
			if !strings.Contains(field, safeValue) {
				t.Fatalf("redacted audit field removed safe value: %q", field)
			}
		}
		if log.OldValue != string(oldValue) || log.NewValue != string(newValue) || log.Metadata != string(metadata) {
			t.Fatalf("ResponseFrom mutated persisted audit data")
		}
	})
}
