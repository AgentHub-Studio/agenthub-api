package admintenant

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteAdminTenantErr_UpstreamLeakSanitized(t *testing.T) {
	cases := []struct {
		name string
		err  string
	}{
		{"keycloak svc cluster URL", `Post "http://keycloak.agenthub.svc.cluster.local:8080/admin/realms/x/users": context deadline exceeded`},
		{"context deadline only", "context deadline exceeded"},
		{"keycloak word match", "keycloak responded with 503"},
		{"dial tcp", "dial tcp 10.0.0.1:8080: connect: connection refused"},
		{"Get http URL", `Get "http://upstream.internal/x": net/http: timeout`},
		{"Delete realm URL", `Delete "http://keycloak.internal/admin/realms/x": EOF`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeAdminTenantErr(rr, "test.op", errors.New(tc.err))
			if rr.Code != 502 {
				t.Fatalf("expected 502, got %d (body=%s)", rr.Code, rr.Body.String())
			}
			body := rr.Body.String()
			if strings.Contains(body, "svc.cluster.local") || strings.Contains(body, "10.0.0.1") || strings.Contains(body, "context deadline") {
				t.Fatalf("body leaks upstream detail: %s", body)
			}
			if !strings.Contains(body, "tenant provisioning service unavailable") {
				t.Fatalf("body missing sanitized message: %s", body)
			}
		})
	}
}

func TestWriteAdminTenantErr_ValidationStaysAs400(t *testing.T) {
	cases := []string{
		"tenantId is required",
		"adminPassword must be at least 8 characters",
		"cannot delete the core tenant",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeAdminTenantErr(rr, "test.op", errors.New(msg))
			if rr.Code != 400 {
				t.Fatalf("expected 400 for %q, got %d", msg, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), msg) {
				t.Fatalf("expected body to contain %q, got %s", msg, rr.Body.String())
			}
		})
	}
}
