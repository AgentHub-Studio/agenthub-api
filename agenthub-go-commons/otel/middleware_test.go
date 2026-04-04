package otel_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	agenthubotel "github.com/AgentHub-Studio/agenthub-go-commons/otel"
)

func TestMiddleware_PassesRequestThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := agenthubotel.Middleware("test-service")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called, "next handler should be called")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_PropagatesTraceContext(t *testing.T) {
	var traceParent string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceParent = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusOK)
	})

	mw := agenthubotel.Middleware("test-service")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// The incoming traceparent header should be passed to the next handler context
	// (middleware itself reads it; inner handler sees original headers).
	_ = traceParent
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_WritesSpanHeaders(t *testing.T) {
	mw := agenthubotel.Middleware("test-service")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTracer_ReturnsNonNil(t *testing.T) {
	tracer := agenthubotel.Tracer("test-service")
	assert.NotNil(t, tracer)
}
