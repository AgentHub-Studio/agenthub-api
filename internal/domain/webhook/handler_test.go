package webhook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/webhook"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockWebhookSvc satisfies the private webhookService interface in webhook.Handler.
type mockWebhookSvc struct {
	webhooks    map[uuid.UUID]webhook.WebhookConfig
	deliveries  map[uuid.UUID][]webhook.WebhookDeliveryLog
	ingestCalls int
	lastPayload []byte
}

func newMockWebhookSvc() *mockWebhookSvc {
	return &mockWebhookSvc{
		webhooks:   make(map[uuid.UUID]webhook.WebhookConfig),
		deliveries: make(map[uuid.UUID][]webhook.WebhookDeliveryLog),
	}
}

func (m *mockWebhookSvc) List(_ context.Context) ([]webhook.WebhookConfig, error) {
	items := make([]webhook.WebhookConfig, 0, len(m.webhooks))
	for _, w := range m.webhooks {
		items = append(items, w)
	}
	return items, nil
}

func (m *mockWebhookSvc) Create(_ context.Context, req webhook.CreateWebhookRequest) (webhook.WebhookConfig, error) {
	id := uuid.New()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	w := webhook.WebhookConfig{
		ID:      id,
		Name:    req.Name,
		URL:     req.URL,
		Events:  req.Events,
		Enabled: enabled,
	}
	m.webhooks[id] = w
	return w, nil
}

func (m *mockWebhookSvc) GetByID(_ context.Context, id uuid.UUID) (webhook.WebhookConfig, error) {
	w, ok := m.webhooks[id]
	if !ok {
		return webhook.WebhookConfig{}, webhook.ErrNotFound
	}
	return w, nil
}

func (m *mockWebhookSvc) Update(_ context.Context, id uuid.UUID, req webhook.UpdateWebhookRequest) (webhook.WebhookConfig, error) {
	w, ok := m.webhooks[id]
	if !ok {
		return webhook.WebhookConfig{}, webhook.ErrNotFound
	}
	if req.Name != nil {
		w.Name = *req.Name
	}
	m.webhooks[id] = w
	return w, nil
}

func (m *mockWebhookSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.webhooks[id]; !ok {
		return webhook.ErrNotFound
	}
	delete(m.webhooks, id)
	return nil
}

func (m *mockWebhookSvc) ListDeliveries(_ context.Context, webhookID uuid.UUID, _ webhook.DeliveryFilter, req pagination.PageRequest) (pagination.Page[webhook.WebhookDeliveryLog], error) {
	logs := m.deliveries[webhookID]
	return pagination.NewPage(logs, int64(len(logs)), req), nil
}

func (m *mockWebhookSvc) IngestWebhook(_ context.Context, token, _ string, payload []byte, _, eventType string) (webhook.WebhookDeliveryLog, error) {
	m.ingestCalls++
	m.lastPayload = append([]byte(nil), payload...)
	// Simulate not-found when token starts with "unknown-"
	if len(token) >= 8 && token[:8] == "unknown-" {
		return webhook.WebhookDeliveryLog{}, webhook.ErrNotFound
	}
	d := webhook.WebhookDeliveryLog{
		ID:        uuid.New(),
		EventType: eventType,
		Status:    "PENDING",
	}
	return d, nil
}

func (m *mockWebhookSvc) SendTest(_ context.Context, id uuid.UUID) (webhook.WebhookDeliveryLog, error) {
	if _, ok := m.webhooks[id]; !ok {
		return webhook.WebhookDeliveryLog{}, webhook.ErrNotFound
	}
	d := webhook.WebhookDeliveryLog{
		ID:        uuid.New(),
		WebhookID: id,
		EventType: "test",
		Status:    "PENDING",
	}
	m.deliveries[id] = append(m.deliveries[id], d)
	return d, nil
}

func setupWebhook() (*chi.Mux, *mockWebhookSvc) {
	return setupWebhookWithRoles("admin")
}

func setupWebhookWithRoles(roles ...string) (*chi.Mux, *mockWebhookSvc) {
	svc := newMockWebhookSvc()
	h := webhook.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(middleware.ContextWithRoles(r.Context(), roles...)))
		})
	})
	h.RegisterRoutes(r)
	return r, svc
}

func setupWebhookWithPublic() (*chi.Mux, *mockWebhookSvc) {
	svc := newMockWebhookSvc()
	h := webhook.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	h.RegisterPublicRoutes(r)
	return r, svc
}

func TestWebhookHandler_List_Success(t *testing.T) {
	r, svc := setupWebhook()
	id := uuid.New()
	svc.webhooks[id] = webhook.WebhookConfig{ID: id, Name: "my-webhook", URL: "https://example.com"}

	req := httptest.NewRequest(http.MethodGet, "/api/webhooks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []webhook.WebhookConfig
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
}

func TestWebhookHandler_Create_Success(t *testing.T) {
	r, _ := setupWebhook()
	body, _ := json.Marshal(webhook.CreateWebhookRequest{
		Name:   "My Webhook",
		URL:    "https://example.com/hook",
		Events: []string{"agent.created"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp webhook.WebhookConfig
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Webhook", resp.Name)
}

func TestWebhookHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupWebhook()
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupWebhook()
		req := httptest.NewRequest(http.MethodPost, "/api/webhooks", bytes.NewBufferString(`{"name":"first","url":"https://example.com","events":["push"]}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.webhooks)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupWebhook()
		id := uuid.New()
		svc.webhooks[id] = webhook.WebhookConfig{ID: id, Name: "original", URL: "https://example.com"}
		req := httptest.NewRequest(http.MethodPatch, "/api/webhooks/"+id.String(), bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.webhooks[id].Name)
	})
}

func TestWebhookHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupWebhook()
	req := httptest.NewRequest(http.MethodGet, "/api/webhooks/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_Delete_Success(t *testing.T) {
	r, svc := setupWebhook()
	id := uuid.New()
	svc.webhooks[id] = webhook.WebhookConfig{ID: id, Name: "to-delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/webhooks/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestWebhookHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupWebhook()
	req := httptest.NewRequest(http.MethodDelete, "/api/webhooks/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupWebhookWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/webhooks"},
		{name: "create", method: http.MethodPost, path: "/api/webhooks", body: `{"name":"webhook","url":"https://example.com","events":["test"]}`},
		{name: "get", method: http.MethodGet, path: "/api/webhooks/" + id},
		{name: "put", method: http.MethodPut, path: "/api/webhooks/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/webhooks/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/webhooks/" + id},
		{name: "deliveries", method: http.MethodGet, path: "/api/webhooks/" + id + "/deliveries"},
		{name: "test", method: http.MethodPost, path: "/api/webhooks/" + id + "/test"},
	}

	for _, tc := range cases {
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

func TestWebhookHandler_SendTest_Success(t *testing.T) {
	r, svc := setupWebhook()
	id := uuid.New()
	svc.webhooks[id] = webhook.WebhookConfig{ID: id, Name: "wh", URL: "https://example.com"}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/"+id.String()+"/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestWebhookHandler_SendTest_NotFound(t *testing.T) {
	r, _ := setupWebhook()
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/"+uuid.New().String()+"/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_ListDeliveries_Success(t *testing.T) {
	r, svc := setupWebhook()
	id := uuid.New()
	svc.webhooks[id] = webhook.WebhookConfig{ID: id, Name: "wh"}

	req := httptest.NewRequest(http.MethodGet, "/api/webhooks/"+id.String()+"/deliveries", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebhookHandler_Ingest_Success(t *testing.T) {
	r, _ := setupWebhookWithPublic()

	body := []byte(`{"ref":"refs/heads/main","commits":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/valid-token/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=abc123")
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "PENDING", resp["status"])
	assert.Equal(t, "push", resp["eventType"])
}

func TestWebhookHandler_Ingest_GitLabSource(t *testing.T) {
	r, _ := setupWebhookWithPublic()

	body := []byte(`{"object_kind":"push"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/valid-token/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "mysecret")
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Push Hook", resp["eventType"])
}

func TestWebhookHandler_Ingest_RejectsBodyAboveOneMiBBeforeService(t *testing.T) {
	r, svc := setupWebhookWithPublic()
	body := bytes.Repeat([]byte("x"), 1<<20+1)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/valid-token/ingest", bytes.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "mysecret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	assert.Zero(t, svc.ingestCalls)
	assert.Empty(t, svc.lastPayload)
}

func TestWebhookHandler_Ingest_AcceptsExactOneMiBWithoutTruncation(t *testing.T) {
	r, svc := setupWebhookWithPublic()
	body := bytes.Repeat([]byte("x"), 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/valid-token/ingest", bytes.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "mysecret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	require.Equal(t, 1, svc.ingestCalls)
	assert.Equal(t, body, svc.lastPayload)
}

func TestWebhookHandler_Ingest_UnknownToken(t *testing.T) {
	r, _ := setupWebhookWithPublic()

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/unknown-abc/ingest", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_Ingest_DefaultEventType(t *testing.T) {
	r, _ := setupWebhookWithPublic()

	// No event header — should default to "push"
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/valid-token/ingest", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "push", resp["eventType"])
}
