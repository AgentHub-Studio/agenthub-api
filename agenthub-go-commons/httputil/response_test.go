package httputil_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/httputil"
)

func TestJSON(t *testing.T) {
	t.Run("writes status and JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		httputil.JSON(w, http.StatusOK, map[string]string{"key": "value"})

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var result map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		assert.Equal(t, "value", result["key"])
	})

	t.Run("writes 201 Created", func(t *testing.T) {
		w := httptest.NewRecorder()
		httputil.JSON(w, http.StatusCreated, map[string]int{"id": 1})

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestError(t *testing.T) {
	t.Run("writes error response with status and message", func(t *testing.T) {
		w := httptest.NewRecorder()
		httputil.Error(w, http.StatusUnprocessableEntity, "validation failed")

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp httputil.ErrorResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Status)
		assert.Equal(t, "validation failed", resp.Message)
	})
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.NotFound(w, "agent")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp httputil.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusNotFound, resp.Status)
	assert.Equal(t, "agent not found", resp.Message)
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.BadRequest(w, "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp httputil.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusBadRequest, resp.Status)
	assert.Equal(t, "invalid input", resp.Message)
}

func TestInternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.InternalServerError(w)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp httputil.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusInternalServerError, resp.Status)
	assert.Equal(t, "internal server error", resp.Message)
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.NoContent(w)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestBindJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	t.Run("decodes valid JSON body", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"test"}`))
		var p payload
		err := httputil.BindJSON(r, &p)
		require.NoError(t, err)
		assert.Equal(t, "test", p.Name)
	})

	t.Run("returns error for nil body", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Body = nil
		var p payload
		err := httputil.BindJSON(r, &p)
		assert.ErrorContains(t, err, "request body is required")
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`not-json`))
		var p payload
		err := httputil.BindJSON(r, &p)
		assert.ErrorContains(t, err, "invalid JSON")
	})
}

func TestValidate(t *testing.T) {
	type dto struct {
		Name string `validate:"required"`
		Age  int    `validate:"min=0,max=150"`
	}

	t.Run("passes valid struct", func(t *testing.T) {
		err := httputil.Validate(&dto{Name: "John", Age: 30})
		assert.NoError(t, err)
	})

	t.Run("fails missing required field", func(t *testing.T) {
		err := httputil.Validate(&dto{Name: "", Age: 30})
		assert.Error(t, err)
	})

	t.Run("fails out of range value", func(t *testing.T) {
		err := httputil.Validate(&dto{Name: "John", Age: 200})
		assert.Error(t, err)
	})
}

func TestBindAndValidate(t *testing.T) {
	type dto struct {
		Name string `json:"name" validate:"required"`
	}

	t.Run("success", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"valid"}`))
		var d dto
		err := httputil.BindAndValidate(r, &d)
		require.NoError(t, err)
		assert.Equal(t, "valid", d.Name)
	})

	t.Run("fails validation after bind", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":""}`))
		var d dto
		err := httputil.BindAndValidate(r, &d)
		assert.Error(t, err)
	})
}
