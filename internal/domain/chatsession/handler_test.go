package chatsession

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_CreateReturnsOpaqueKeyAndGetReturnsPayload(t *testing.T) {
	handler := NewHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body := bytes.NewBufferString(`{"token":"jwt-secret","tenantId":"test","apiBaseUrl":"https://api.cezar.dev"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/session", body)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	require.Equal(t, http.StatusOK, createRec.Code)
	var created map[string]string
	require.NoError(t, json.NewDecoder(createRec.Body).Decode(&created))
	require.NotEmpty(t, created["key"])

	raw, err := base64.RawURLEncoding.DecodeString(created["key"])
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "jwt-secret")

	getReq := httptest.NewRequest(http.MethodGet, "/api/session/"+created["key"], nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)

	require.Equal(t, http.StatusOK, getRec.Code)
	var payload sessionPayload
	require.NoError(t, json.NewDecoder(getRec.Body).Decode(&payload))
	assert.Equal(t, "jwt-secret", payload.Token)
	assert.Equal(t, "test", payload.TenantID)
}

func TestHandler_RefreshTokenKeepsOpaqueKey(t *testing.T) {
	handler := NewHandler()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	createReq := httptest.NewRequest(http.MethodPost, "/api/session", bytes.NewBufferString(`{"token":"old","tenantId":"test","apiBaseUrl":"https://api.cezar.dev"}`))
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created map[string]string
	require.NoError(t, json.NewDecoder(createRec.Body).Decode(&created))
	key := created["key"]

	refreshReq := httptest.NewRequest(http.MethodPut, "/api/session/"+key+"/token", bytes.NewBufferString(`{"token":"new"}`))
	refreshRec := httptest.NewRecorder()
	router.ServeHTTP(refreshRec, refreshReq)

	require.Equal(t, http.StatusOK, refreshRec.Code)
	var refreshed map[string]string
	require.NoError(t, json.NewDecoder(refreshRec.Body).Decode(&refreshed))
	assert.Equal(t, key, refreshed["key"])

	getReq := httptest.NewRequest(http.MethodGet, "/api/session/"+key, nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)

	var payload sessionPayload
	require.NoError(t, json.NewDecoder(getRec.Body).Decode(&payload))
	assert.Equal(t, "new", payload.Token)
}

func TestHandler_CreateAndRefreshRejectTrailingJSONWithoutSessionEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		handler := NewHandler()
		router := chi.NewRouter()
		handler.RegisterRoutes(router)
		req := httptest.NewRequest(http.MethodPost, "/api/session", bytes.NewBufferString(`{"token":"jwt","tenantId":"test","apiBaseUrl":"https://api.cezar.dev"} {"token":"ignored"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Empty(t, handler.sessions)
	})

	t.Run("refresh", func(t *testing.T) {
		handler := NewHandler()
		handler.store("opaque", sessionPayload{Token: "old", TenantID: "test", APIBaseURL: "https://api.cezar.dev"})
		router := chi.NewRouter()
		handler.RegisterRoutes(router)
		req := httptest.NewRequest(http.MethodPut, "/api/session/opaque/token", bytes.NewBufferString(`{"token":"new"} {"token":"ignored"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		payload, ok := handler.load("opaque")
		require.True(t, ok)
		assert.Equal(t, "old", payload.Token)
	})
}

func TestHandler_ExpiredSessionReturnsNotFound(t *testing.T) {
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	handler := NewHandler()
	handler.now = func() time.Time { return now }
	handler.ttl = time.Minute

	key := "opaque"
	handler.store(key, sessionPayload{Token: "jwt", TenantID: "test", APIBaseURL: "https://api.cezar.dev"})

	now = now.Add(2 * time.Minute)
	_, ok := handler.load(key)

	assert.False(t, ok)
	assert.Empty(t, handler.sessions)
}
