package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

func TestMaxURLBytes_UsesExactRequestTargetLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		limit  int
		status int
	}{
		{
			name:   "path without query at exact limit",
			target: "/alpha",
			limit:  len("/alpha"),
			status: http.StatusNoContent,
		},
		{
			name:   "query at exact limit",
			target: "/alpha?q=1",
			limit:  len("/alpha?q=1"),
			status: http.StatusNoContent,
		},
		{
			name:   "escaped path is measured as received",
			target: "/a%20b",
			limit:  len("/a%20b") - 1,
			status: http.StatusRequestURITooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			handler := middleware.MaxURLBytes(tt.limit)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.status, rec.Code)
			assert.Equal(t, tt.status == http.StatusNoContent, called)
		})
	}
}

func TestMaxURLBytes_RejectsOversizedRequestWithJSONError(t *testing.T) {
	t.Parallel()

	handler := middleware.MaxURLBytes(len("/ok"))(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("next handler must not receive an oversized request URI")
	}))

	req := httptest.NewRequest(http.MethodGet, "/toolong", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestURITooLong, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "request URI too long", body["error"])
}
