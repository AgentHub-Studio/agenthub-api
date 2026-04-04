//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AgentHub-Studio/agenthub-go-commons/auth"
	"github.com/AgentHub-Studio/agenthub-go-commons/pagination"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseIntegration tests DB connection, schema setup, and pagination.
func TestDatabaseIntegration(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)

	ctx := context.Background()

	// Basic query
	var result int
	err := pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result)

	// Create tenant schema
	testutil.CreateTenantSchema(t, pool, "test-tenant")

	// Verify schema exists
	var schemaName string
	err = pool.QueryRow(ctx,
		"SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1",
		"ah_test-tenant",
	).Scan(&schemaName)
	require.NoError(t, err)
	assert.Equal(t, "ah_test-tenant", schemaName)
}

// TestPaginationIntegration tests Page[T] with real query results.
func TestPaginationIntegration(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	// Create test table
	testutil.MustExec(t, pool, `CREATE TABLE IF NOT EXISTS test_items (
		id   SERIAL PRIMARY KEY,
		name TEXT NOT NULL
	)`)

	// Insert 25 items
	for i := 0; i < 25; i++ {
		testutil.MustExec(t, pool, "INSERT INTO test_items (name) VALUES ($1)", fmt.Sprintf("item-%d", i))
	}

	// Test pagination — page 0, size 10
	req := httptest.NewRequest("GET", "/?page=0&size=10", nil)
	pageReq := pagination.FromRequest(req)

	rows, err := pool.Query(ctx, "SELECT name FROM test_items ORDER BY id LIMIT $1 OFFSET $2",
		pageReq.Size, pageReq.Offset())
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())

	page := pagination.NewPage(names, pageReq, 25)
	assert.Equal(t, 10, len(page.Content))
	assert.Equal(t, int64(25), page.TotalElements)
	assert.Equal(t, 3, page.TotalPages)
	assert.True(t, page.First)
	assert.False(t, page.Last)
}

// TestAuthMiddlewareIntegration tests JWT middleware with a mock JWKS server.
func TestAuthMiddlewareIntegration(t *testing.T) {
	// Mock JWKS server that returns 404 (simulates missing tenant)
	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer jwksSrv.Close()

	cfg := auth.Config{KeycloakBaseURL: jwksSrv.URL}
	middleware := auth.Middleware(cfg)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without token — should return 401
	r := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Request with malformed token — should return 401
	r = httptest.NewRequest("GET", "/api/test", nil)
	r.Header.Set("Authorization", "Bearer invalid.token.here")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
