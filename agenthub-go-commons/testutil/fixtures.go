package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// NewTestServer creates an httptest.Server with the given chi router.
// It is closed when t finishes.
func NewTestServer(t *testing.T, r chi.Router) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// WithContext returns a copy of r with ctx set.
func WithContext(r *http.Request, ctx context.Context) *http.Request {
	return r.WithContext(ctx)
}

// RequireStatus asserts that resp has the expected status code.
func RequireStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("testutil: expected status %d, got %d", expected, resp.StatusCode)
	}
}
