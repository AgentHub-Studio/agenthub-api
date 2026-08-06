package probe

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzProbeSecretRedactorHidesConfiguredSecrets(f *testing.F) {
	for _, seed := range [][3]string{
		{"bearer", "probe-token-123", "header-secret-456"},
		{"basic", "dXNlcjpwYXNz", "Bearer header-token"},
		{"", "raw-token", "Key raw-header-value"},
		{"Bearer", "token with spaces", "value with spaces"},
	} {
		f.Add(seed[0], seed[1], seed[2])
	}

	f.Fuzz(func(t *testing.T, authType, authToken, headerValue string) {
		if len(authType) > 64 || len(authToken) > 256 || len(headerValue) > 256 || !utf8.ValidString(authType) || !utf8.ValidString(authToken) || !utf8.ValidString(headerValue) {
			t.Skip()
		}

		token := strings.TrimSpace(authToken)
		header := strings.TrimSpace(headerValue)
		if token == "" && header == "" {
			t.Skip()
		}
		if isRedactionMaskLike(token) || isRedactionMaskLike(header) {
			t.Skip()
		}

		headers, err := json.Marshal(map[string]string{"X-API-Key": headerValue})
		if err != nil {
			t.Fatal(err)
		}

		secrets := []string{header}
		if token != "" {
			secrets = append(secrets, token, "Bearer "+token, "Basic "+token)
		}

		redactor := newProbeSecretRedactor(authType, authToken, headers)
		for _, secret := range secrets {
			if secret == "" {
				continue
			}
			redacted := redactor.redact(secret)
			if strings.Contains(redacted, secret) {
				t.Fatalf("configured secret leaked after redaction: %q", redacted)
			}
		}
	})
}

func isRedactionMaskLike(value string) bool {
	return value != "" && strings.Trim(value, "*") == ""
}
