package prompttemplate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/prompttemplate"
)

// stubService wraps a real service backed by a stub repo for unit tests.
// For handler tests we need a lightweight in-memory approach.
// We test the handler layer by verifying HTTP status codes and response format.

func setupRouter(handler *prompttemplate.Handler) *chi.Mux {
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

// Note: list/CRUD tests that need a real service are skipped when no DB is available.
// We only test input validation and routing in handler_test.go.

func TestHandler_Get_InvalidID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/prompt-templates/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Create_InvalidBody(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/prompt-templates", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Create_MissingRequiredFields(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	body := prompttemplate.CreateRequest{
		Name: "", // empty
		Slug: "test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/prompt-templates", bytes.NewReader(b))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_Update_InvalidID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPut, "/api/prompt-templates/bad-id", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Delete_InvalidID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/prompt-templates/bad-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ListByAgent_InvalidAgentID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/not-uuid/prompt-templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CreateForAgent_InvalidBody(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/00000000-0000-0000-0000-000000000001/prompt-templates", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResponseFrom(t *testing.T) {
	tmpl := prompttemplate.PromptTemplate{
		Name:     "Test",
		Slug:     "test",
		Category: prompttemplate.CategoryRAG,
		Content:  "Hello",
	}
	resp := prompttemplate.ResponseFrom(tmpl)
	require.Equal(t, "Test", resp.Name)
	require.Equal(t, "rag", resp.Category)
	require.Equal(t, "Hello", resp.Content)
}
