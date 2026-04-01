package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

func TestChain_Public_NoAuthRequired(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})

	// Public chain should not reject requests without Authorization header.
	handler := applyMiddlewares(chain.Public(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestChain_Protected_RejectsNoToken(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})

	// Protected chain must reject requests without an Authorization header.
	handler := applyMiddlewares(chain.Protected(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestChain_Protected_RejectsInvalidToken(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})

	handler := applyMiddlewares(chain.Protected(), okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestChain_Public_HasCORSHeader(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"https://app.example.com"})

	handler := applyMiddlewares(chain.Public(), okHandler())
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// CORS header should be present.
	require.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestChain_New_ReturnsNonNil(t *testing.T) {
	chain := middleware.New("http://keycloak:8080", []string{"*"})
	assert.NotNil(t, chain)
	assert.NotEmpty(t, chain.Public())
	assert.NotEmpty(t, chain.Protected())
}

// applyMiddlewares wraps the given handler with each middleware in order.
func applyMiddlewares(middlewares []func(http.Handler) http.Handler, h http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// okHandler returns an HTTP handler that always responds 200 OK.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
