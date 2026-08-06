package middleware

import (
	"strings"
	"testing"
)

func TestBuildRealmJWKSURL_EscapesRealmAndPreservesBasePath(t *testing.T) {
	got, err := buildRealmJWKSURL("https://keycloak.example/auth/", "tenant one")
	if err != nil {
		t.Fatalf("buildRealmJWKSURL returned error: %v", err)
	}

	want := "https://keycloak.example/auth/realms/tenant%20one/protocol/openid-connect/certs"
	if got != want {
		t.Fatalf("buildRealmJWKSURL = %q, want %q", got, want)
	}
}

func TestBuildRealmJWKSURL_RejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		realm   string
	}{
		{name: "userinfo", baseURL: "https://user:pass@keycloak.example", realm: "tenant"},
		{name: "unsupported scheme", baseURL: "file:///tmp/keycloak", realm: "tenant"},
		{name: "query", baseURL: "https://keycloak.example?target=other", realm: "tenant"},
		{name: "blank realm", baseURL: "https://keycloak.example", realm: ""},
		{name: "realm path separator", baseURL: "https://keycloak.example", realm: "../master"},
		{name: "control char", baseURL: "https://keycloak.example", realm: "tenant\nother"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildRealmJWKSURL(tc.baseURL, tc.realm)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "invalid") {
				t.Fatalf("expected invalid input error, got %v", err)
			}
		})
	}
}
