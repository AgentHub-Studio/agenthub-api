package settings_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
)

// mockSettingsSvc implements settings.Service for handler tests.
type mockSettingsSvc struct {
	data map[string]settings.SettingResponse
}

func newMockSettingsSvc() *mockSettingsSvc {
	return &mockSettingsSvc{data: make(map[string]settings.SettingResponse)}
}

func (m *mockSettingsSvc) List(_ context.Context) ([]settings.SettingResponse, error) {
	items := make([]settings.SettingResponse, 0, len(m.data))
	for _, v := range m.data {
		items = append(items, v)
	}
	return items, nil
}

func (m *mockSettingsSvc) Get(_ context.Context, key string) (settings.SettingResponse, error) {
	s, ok := m.data[key]
	if !ok {
		return settings.SettingResponse{}, settings.ErrNotFound
	}
	return s, nil
}

func (m *mockSettingsSvc) Upsert(_ context.Context, key string, req settings.UpdateSettingRequest) (settings.SettingResponse, error) {
	s := settings.SettingResponse{Key: key, Value: req.Value}
	m.data[key] = s
	return s, nil
}

func (m *mockSettingsSvc) Delete(_ context.Context, key string) error {
	if _, ok := m.data[key]; !ok {
		return settings.ErrNotFound
	}
	delete(m.data, key)
	return nil
}

func setupSettings() (*chi.Mux, *mockSettingsSvc) {
	svc := newMockSettingsSvc()
	h := settings.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterProtectedRoutes(r)
	return r, svc
}

func TestSettingsHandler_List_Success(t *testing.T) {
	r, svc := setupSettings()
	svc.data["feature.x"] = settings.SettingResponse{Key: "feature.x", Value: json.RawMessage(`true`)}

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []settings.SettingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
}

func TestSettingsHandler_List_Empty(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_Get_Success(t *testing.T) {
	r, svc := setupSettings()
	svc.data["my.key"] = settings.SettingResponse{Key: "my.key", Value: json.RawMessage(`"hello"`)}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/my.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_Get_NotFound(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/missing.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSettingsHandler_Upsert_Success(t *testing.T) {
	r, _ := setupSettings()
	body, _ := json.Marshal(settings.UpdateSettingRequest{Value: json.RawMessage(`42`)})
	req := httptest.NewRequest(http.MethodPut, "/api/settings/my.key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSettingsHandler_Upsert_InvalidBody(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodPut, "/api/settings/my.key", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSettingsHandler_Delete_Success(t *testing.T) {
	r, svc := setupSettings()
	svc.data["del.key"] = settings.SettingResponse{Key: "del.key"}

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/del.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestSettingsHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupSettings()
	req := httptest.NewRequest(http.MethodDelete, "/api/settings/missing.key", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// mockImpactAssessor implements impactAssessor for handler tests.
type mockImpactAssessor struct {
	count int64
	err   error
}

func (m *mockImpactAssessor) CountPublishedWithoutProvider(_ context.Context) (int64, error) {
	return m.count, m.err
}

func setupSettingsWithAssessor(assessor settings.ImpactAssessor) (*chi.Mux, *mockSettingsSvc) {
	svc := &mockSettingsSvc{data: make(map[string]settings.SettingResponse)}
	h := settings.NewHandler(svc).WithImpactAssessor(assessor)
	r := chi.NewRouter()
	h.RegisterProtectedRoutes(r)
	return r, svc
}

// TestSettingsHandler_ProviderImpact_WithAssessor verifies the endpoint returns
// the count from the assessor (ACT-F3-14).
func TestSettingsHandler_ProviderImpact_WithAssessor(t *testing.T) {
	assessor := &mockImpactAssessor{count: 7}
	r, _ := setupSettingsWithAssessor(assessor)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/provider-impact", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp settings.ProviderImpactResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, int64(7), resp.AffectedAgents)
}

// TestSettingsHandler_ProviderImpact_NoAssessor returns zero when no assessor is set.
func TestSettingsHandler_ProviderImpact_NoAssessor(t *testing.T) {
	r, _ := setupSettings()

	req := httptest.NewRequest(http.MethodGet, "/api/settings/provider-impact", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp settings.ProviderImpactResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, int64(0), resp.AffectedAgents)
}
