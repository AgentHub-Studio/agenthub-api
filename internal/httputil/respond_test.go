package httputil_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
)

func TestJSON_WritesStatusAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	httputil.JSON(rec, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "value", body["key"])
}

func TestError_WritesStatusAndMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	httputil.Error(rec, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "invalid input", body["message"])
	assert.Equal(t, float64(400), body["status"])
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	httputil.NotFound(rec, "agent not found")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	httputil.BadRequest(rec, "bad request")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestForbidden(t *testing.T) {
	rec := httptest.NewRecorder()
	httputil.Forbidden(rec, "forbidden")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestInternalServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	httputil.InternalServerError(rec, "server error")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestConflict(t *testing.T) {
	rec := httptest.NewRecorder()
	httputil.Conflict(rec, "already exists")
	assert.Equal(t, http.StatusConflict, rec.Code)
}
