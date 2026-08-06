package middleware_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

func TestMaxBodyBytes_EnforcesReadLimitWithoutRejectingExactLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		limit        int64
		wantTooLarge bool
	}{
		{
			name:  "exact limit reaches handler",
			body:  "12345",
			limit: 5,
		},
		{
			name:         "body above limit fails while reading",
			body:         "123456",
			limit:        5,
			wantTooLarge: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			handler := middleware.MaxBodyBytes(tt.limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, err := io.ReadAll(r.Body)
				if tt.wantTooLarge {
					var maxBytesErr *http.MaxBytesError
					require.ErrorAs(t, err, &maxBytesErr)
					assert.Equal(t, tt.limit, maxBytesErr.Limit)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				require.NoError(t, err)
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.True(t, called)
			if tt.wantTooLarge {
				assert.Equal(t, http.StatusBadRequest, rec.Code)
				return
			}
			assert.Equal(t, http.StatusNoContent, rec.Code)
		})
	}
}

func TestSecurityHeaders_ArePresentOnErrorResponse(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/protected", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
}

func TestProxyServiceRequired_IsFailClosedAndReturnsJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		roles  []string
		status int
	}{
		{name: "missing claims", status: http.StatusForbidden},
		{name: "unrelated role", roles: []string{"admin"}, status: http.StatusForbidden},
		{name: "proxy service role", roles: []string{"PROXY_SERVICE"}, status: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			handler := middleware.ProxyServiceRequired(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/proxy/datasources", nil)
			if tt.roles != nil {
				req = req.WithContext(middleware.ContextWithRoles(req.Context(), tt.roles...))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.status, rec.Code)
			assert.Equal(t, tt.status == http.StatusNoContent, called)
			if tt.status == http.StatusForbidden {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				var body map[string]string
				require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
				assert.Equal(t, "forbidden: PROXY_SERVICE role required", body["error"])
			}
		})
	}
}

func TestRequireCoreTenant_IsFailClosedAndReturnsJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tenant string
		status int
	}{
		{name: "missing tenant", status: http.StatusForbidden},
		{name: "non core tenant", tenant: "customer-a", status: http.StatusForbidden},
		{name: "core tenant", tenant: middleware.CoreTenantID, status: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			handler := middleware.RequireCoreTenant(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/admin/tenants", nil)
			if tt.tenant != "" {
				req = req.WithContext(tenant.NewContext(req.Context(), tt.tenant))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.status, rec.Code)
			assert.Equal(t, tt.status == http.StatusNoContent, called)
			if tt.status == http.StatusForbidden {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				var body map[string]string
				require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
				assert.Equal(t, "forbidden: core tenant only", body["error"])
			}
		})
	}
}

func TestMaxBodyBytes_PreservesNilBody(t *testing.T) {
	t.Parallel()

	handler := middleware.MaxBodyBytes(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Body = nil
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestMaxBodyBytes_ReturnsMaxBytesError(t *testing.T) {
	t.Parallel()

	handler := middleware.MaxBodyBytes(3)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		var maxBytesErr *http.MaxBytesError
		require.True(t, errors.As(err, &maxBytesErr))
		assert.Equal(t, int64(3), maxBytesErr.Limit)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader("1234")))
}
