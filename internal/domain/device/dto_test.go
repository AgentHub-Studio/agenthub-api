package device

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseFrom_RedactsSensitiveMetadata(t *testing.T) {
	const apiKeySecret = "device-metadata-api-key-sentinel"
	const authorizationSecret = "Bearer device-metadata-authorization-sentinel"
	const refreshTokenSecret = "device-metadata-refresh-token-sentinel"
	const clientCredentialSecret = "device-metadata-client-credential-sentinel"
	metadata := json.RawMessage(`{
		"firmware": "1.2.3",
		"apiKey": "device-metadata-api-key-sentinel",
		"headers": {"Authorization": "Bearer device-metadata-authorization-sentinel"},
		"nested": {"refresh_token": "device-metadata-refresh-token-sentinel", "clientCredential": "device-metadata-client-credential-sentinel", "location": "lab-a"}
	}`)
	device := Device{Metadata: metadata}

	response := responseFrom(device)
	data, err := json.Marshal(response)
	require.NoError(t, err)
	body := string(data)

	assert.NotContains(t, body, apiKeySecret)
	assert.NotContains(t, body, authorizationSecret)
	assert.NotContains(t, body, refreshTokenSecret)
	assert.NotContains(t, body, clientCredentialSecret)
	assert.NotContains(t, body, "apiKey")
	assert.NotContains(t, body, "Authorization")
	assert.NotContains(t, body, "refresh_token")
	assert.NotContains(t, body, "clientCredential")
	assert.Contains(t, body, "firmware")
	assert.Contains(t, body, "lab-a")
	assert.Equal(t, string(metadata), string(device.Metadata), "the public DTO must not mutate persisted metadata")
}

func FuzzResponseFromRedactsSensitiveMetadata(f *testing.F) {
	f.Add("device-metadata-secret", "safe-device-location")
	f.Add("credential-seed", "firmware-version")

	f.Fuzz(func(t *testing.T, secretSeed, safeSeed string) {
		secretSum := sha256.Sum256([]byte(secretSeed))
		safeSum := sha256.Sum256([]byte(safeSeed))
		secret := "device-metadata-secret-" + hex.EncodeToString(secretSum[:])
		safeValue := "safe-metadata-value-" + hex.EncodeToString(safeSum[:])
		metadata, err := json.Marshal(map[string]any{
			"firmware": "1.2.3",
			"api_key":  secret,
			"headers": map[string]string{
				"Authorization": "Bearer " + secret,
			},
			"nested": map[string]any{
				"refreshToken":     secret,
				"clientCredential": secret,
				"location":         safeValue,
			},
		})
		require.NoError(t, err)
		device := Device{Metadata: metadata}

		response := responseFrom(device)
		public := string(response.Metadata)
		if !json.Valid(response.Metadata) {
			t.Fatalf("redacted metadata is not valid JSON: %q", public)
		}
		if strings.Contains(public, secret) {
			t.Fatalf("public metadata leaked secret: %q", public)
		}
		for _, sensitiveKey := range []string{"api_key", "Authorization", "refreshToken", "clientCredential"} {
			if strings.Contains(public, sensitiveKey) {
				t.Fatalf("public metadata retained sensitive key %q: %q", sensitiveKey, public)
			}
		}
		if !strings.Contains(public, safeValue) {
			t.Fatalf("public metadata removed safe value: %q", public)
		}
		if string(device.Metadata) != string(metadata) {
			t.Fatalf("responseFrom mutated persisted metadata")
		}
	})
}
