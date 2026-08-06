package memory_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

// mockMemorySvc satisfies the private memoryService interface in memory.Handler.
type mockMemorySvc struct {
	entries     map[string]memory.AgentMemory // key: agentID+":"+key
	recallCalls int
}

func newMockMemorySvc() *mockMemorySvc {
	return &mockMemorySvc{entries: make(map[string]memory.AgentMemory)}
}

func (m *mockMemorySvc) List(_ context.Context, agentID uuid.UUID, _ *string) ([]memory.AgentMemory, error) {
	var items []memory.AgentMemory
	prefix := agentID.String() + ":"
	for k, v := range m.entries {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			items = append(items, v)
		}
	}
	return items, nil
}

func (m *mockMemorySvc) Upsert(_ context.Context, agentID uuid.UUID, key string, req memory.UpsertMemoryRequest) (memory.AgentMemory, error) {
	e := memory.AgentMemory{
		ID:      uuid.New(),
		AgentID: agentID,
		Key:     key,
		Value:   req.Value,
	}
	m.entries[entryKey(agentID, key)] = e
	return e, nil
}

func (m *mockMemorySvc) GetByKey(_ context.Context, agentID uuid.UUID, _ *string, key string) (memory.AgentMemory, error) {
	e, ok := m.entries[entryKey(agentID, key)]
	if !ok {
		return memory.AgentMemory{}, memory.ErrNotFound
	}
	return e, nil
}

func (m *mockMemorySvc) DeleteByKey(_ context.Context, agentID uuid.UUID, _ *string, key string) error {
	k := entryKey(agentID, key)
	if _, ok := m.entries[k]; !ok {
		return memory.ErrNotFound
	}
	delete(m.entries, k)
	return nil
}

func (m *mockMemorySvc) ClearByAgent(_ context.Context, agentID uuid.UUID) error {
	prefix := agentID.String() + ":"
	for k := range m.entries {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			delete(m.entries, k)
		}
	}
	return nil
}

func (m *mockMemorySvc) Recall(_ context.Context, agentID uuid.UUID, req memory.RecallRequest) ([]memory.MemoryRecallResult, error) {
	m.recallCalls++
	return []memory.MemoryRecallResult{}, nil
}

func (m *mockMemorySvc) ListByType(_ context.Context, agentID uuid.UUID, _ *string, memType memory.MemoryType) ([]memory.AgentMemory, error) {
	var items []memory.AgentMemory
	prefix := agentID.String() + ":"
	for k, v := range m.entries {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix && v.MemoryType == memType {
			items = append(items, v)
		}
	}
	return items, nil
}

func (m *mockMemorySvc) SearchByText(_ context.Context, agentID uuid.UUID, query string, limit int) ([]memory.AgentMemory, error) {
	var items []memory.AgentMemory
	prefix := agentID.String() + ":"
	for k, v := range m.entries {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			if strings.Contains(v.Key, query) || strings.Contains(string(v.Value), query) {
				items = append(items, v)
			}
		}
	}
	return items, nil
}

func (m *mockMemorySvc) Stats(_ context.Context, agentID uuid.UUID) (memory.MemoryStats, error) {
	counts := make(map[string]int)
	total := 0
	prefix := agentID.String() + ":"
	for k, v := range m.entries {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			counts[string(v.MemoryType)]++
			total++
		}
	}
	return memory.MemoryStats{Total: total, ByType: counts}, nil
}

func (m *mockMemorySvc) BulkUpsert(_ context.Context, agentID uuid.UUID, entries []memory.BulkMemoryEntry) (int, error) {
	stored := 0
	for _, e := range entries {
		m.entries[entryKey(agentID, e.Key)] = memory.AgentMemory{
			ID: uuid.New(), AgentID: agentID, Key: e.Key, Value: e.Value,
		}
		stored++
	}
	return stored, nil
}

func setupMemory() (*chi.Mux, *mockMemorySvc) {
	return setupMemoryWithRoles("admin")
}

func setupMemoryWithRoles(roles ...string) (*chi.Mux, *mockMemorySvc) {
	svc := newMockMemorySvc()
	h := memory.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r, svc
}

func TestMemoryHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupMemoryWithRoles("user")
	agentID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/agents/" + agentID + "/memory"},
		{name: "upsert", method: http.MethodPut, path: "/api/agents/" + agentID + "/memory/key", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/agents/" + agentID + "/memory/key"},
		{name: "delete by key", method: http.MethodDelete, path: "/api/agents/" + agentID + "/memory/key"},
		{name: "clear", method: http.MethodDelete, path: "/api/agents/" + agentID + "/memory"},
		{name: "recall", method: http.MethodPost, path: "/api/agents/" + agentID + "/memory/recall", body: `{}`},
		{name: "search", method: http.MethodGet, path: "/api/agents/" + agentID + "/memory/search?q=query"},
		{name: "stats", method: http.MethodGet, path: "/api/agents/" + agentID + "/memory/stats"},
		{name: "bulk upsert", method: http.MethodPost, path: "/api/agents/" + agentID + "/memory/bulk", body: `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
}

func TestMemoryHandler_List_Success(t *testing.T) {
	r, svc := setupMemory()
	agentID := uuid.New()
	svc.entries[entryKey(agentID, "lang")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "lang", Value: []byte(`"en"`)}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/memory", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []memory.AgentMemory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
}

func TestMemoryHandler_Upsert_Success(t *testing.T) {
	r, _ := setupMemory()
	agentID := uuid.New()
	body, _ := json.Marshal(memory.UpsertMemoryRequest{Value: json.RawMessage(`"hello"`)})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/memory/greeting", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMemoryHandler_Upsert_InvalidBody(t *testing.T) {
	r, _ := setupMemory()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/memory/greeting", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMemoryHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("upsert", func(t *testing.T) {
		r, svc := setupMemory()
		agentID := uuid.New()
		svc.entries[entryKey(agentID, "profile")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "profile", Value: json.RawMessage(`"original"`)}
		req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/memory/profile", bytes.NewBufferString(`{"value":"changed"}{"value":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, `"original"`, string(svc.entries[entryKey(agentID, "profile")].Value))
	})

	t.Run("recall", func(t *testing.T) {
		r, svc := setupMemory()
		agentID := uuid.New()
		req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/memory/recall", bytes.NewBufferString(`{"embedding":[1],"limit":1}{"limit":10}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, svc.recallCalls)
	})

	t.Run("bulk upsert", func(t *testing.T) {
		r, svc := setupMemory()
		agentID := uuid.New()
		req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/memory/bulk", bytes.NewBufferString(`[{"key":"first","value":"value","memoryType":"user"}][{"key":"ignored","value":"value","memoryType":"user"}]`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.entries)
	})
}

func TestMemoryHandler_GetByKey_NotFound(t *testing.T) {
	r, _ := setupMemory()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/memory/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMemoryHandler_DeleteByKey_Success(t *testing.T) {
	r, svc := setupMemory()
	agentID := uuid.New()
	svc.entries[entryKey(agentID, "lang")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "lang"}

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+agentID.String()+"/memory/lang", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestMemoryHandler_DeleteByKey_NotFound(t *testing.T) {
	r, _ := setupMemory()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+agentID.String()+"/memory/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMemoryHandler_Clear_Success(t *testing.T) {
	r, svc := setupMemory()
	agentID := uuid.New()
	svc.entries[entryKey(agentID, "k1")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "k1"}

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+agentID.String()+"/memory", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestMemoryHandler_ListByType(t *testing.T) {
	r, svc := setupMemory()
	agentID := uuid.New()
	svc.entries[entryKey(agentID, "pref")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "pref", MemoryType: memory.MemoryTypeUser, Value: []byte(`"x"`)}
	svc.entries[entryKey(agentID, "rule")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "rule", MemoryType: memory.MemoryTypeFeedback, Value: []byte(`"y"`)}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/memory?memoryType=user", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []memory.AgentMemory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
	assert.Equal(t, "pref", items[0].Key)
}

func TestMemoryHandler_Search(t *testing.T) {
	r, svc := setupMemory()
	agentID := uuid.New()
	svc.entries[entryKey(agentID, "go_stack")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "go_stack", Value: []byte(`"Go and PostgreSQL"`)}
	svc.entries[entryKey(agentID, "other")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "other", Value: []byte(`"unrelated"`)}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/memory/search?q=go_stack", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []memory.AgentMemory
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
}

func TestMemoryHandler_Search_MissingQuery(t *testing.T) {
	r, _ := setupMemory()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/memory/search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMemoryHandler_Stats(t *testing.T) {
	r, svc := setupMemory()
	agentID := uuid.New()
	svc.entries[entryKey(agentID, "k1")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "k1", MemoryType: memory.MemoryTypeUser, Value: []byte(`"a"`)}
	svc.entries[entryKey(agentID, "k2")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "k2", MemoryType: memory.MemoryTypeUser, Value: []byte(`"b"`)}
	svc.entries[entryKey(agentID, "k3")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "k3", MemoryType: memory.MemoryTypeFeedback, Value: []byte(`"c"`)}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/memory/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var stats memory.MemoryStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 2, stats.ByType["user"])
	assert.Equal(t, 1, stats.ByType["feedback"])
}

func TestMemoryHandler_SummaryAlias(t *testing.T) {
	r, svc := setupMemory()
	agentID := uuid.New()
	svc.entries[entryKey(agentID, "k1")] = memory.AgentMemory{ID: uuid.New(), AgentID: agentID, Key: "k1", MemoryType: memory.MemoryTypeProject, Value: []byte(`"a"`)}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/memories/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var stats memory.MemoryStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, 1, stats.Total)
	assert.Equal(t, 1, stats.ByType["project"])
}

func TestMemoryHandler_BulkUpsert(t *testing.T) {
	r, _ := setupMemory()
	agentID := uuid.New()
	entries := []memory.BulkMemoryEntry{
		{Key: "k1", Value: json.RawMessage(`"value1"`), MemoryType: "user"},
		{Key: "k2", Value: json.RawMessage(`"value2"`), MemoryType: "feedback"},
	}
	body, _ := json.Marshal(entries)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/memory/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]int
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 2, result["stored"])
	assert.Equal(t, 2, result["total"])
}

func TestMemoryHandler_BulkUpsert_Empty(t *testing.T) {
	r, _ := setupMemory()
	agentID := uuid.New()
	body, _ := json.Marshal([]memory.BulkMemoryEntry{})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/memory/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
