package webhook_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/webhook"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockWebhookRepo struct {
	configs    map[uuid.UUID]webhook.WebhookConfig
	deliveries []webhook.WebhookDeliveryLog
}

func newMockRepo() *mockWebhookRepo {
	return &mockWebhookRepo{configs: make(map[uuid.UUID]webhook.WebhookConfig)}
}

func (m *mockWebhookRepo) List(_ context.Context) ([]webhook.WebhookConfig, error) {
	out := make([]webhook.WebhookConfig, 0, len(m.configs))
	for _, c := range m.configs {
		out = append(out, c)
	}
	return out, nil
}

func (m *mockWebhookRepo) Create(_ context.Context, w webhook.WebhookConfig) (webhook.WebhookConfig, error) {
	w.ID = uuid.New()
	m.configs[w.ID] = w
	return w, nil
}

func (m *mockWebhookRepo) GetByID(_ context.Context, id uuid.UUID) (webhook.WebhookConfig, error) {
	w, ok := m.configs[id]
	if !ok {
		return webhook.WebhookConfig{}, webhook.ErrNotFound
	}
	return w, nil
}

func (m *mockWebhookRepo) Update(_ context.Context, w webhook.WebhookConfig) (webhook.WebhookConfig, error) {
	if _, ok := m.configs[w.ID]; !ok {
		return webhook.WebhookConfig{}, webhook.ErrNotFound
	}
	m.configs[w.ID] = w
	return w, nil
}

func (m *mockWebhookRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.configs[id]; !ok {
		return webhook.ErrNotFound
	}
	delete(m.configs, id)
	return nil
}

func (m *mockWebhookRepo) ListDeliveries(_ context.Context, webhookID uuid.UUID, filter webhook.DeliveryFilter, _ pagination.PageRequest) ([]webhook.WebhookDeliveryLog, int64, error) {
	var out []webhook.WebhookDeliveryLog
	for _, d := range m.deliveries {
		if d.WebhookID != webhookID {
			continue
		}
		if filter.Status != "" && d.Status != filter.Status {
			continue
		}
		if filter.EventType != "" && d.EventType != filter.EventType {
			continue
		}
		out = append(out, d)
	}
	return out, int64(len(out)), nil
}

func (m *mockWebhookRepo) GetBySecret(_ context.Context, secret string) (webhook.WebhookConfig, error) {
	for _, c := range m.configs {
		if c.Secret != nil && *c.Secret == secret && c.Enabled {
			return c, nil
		}
	}
	return webhook.WebhookConfig{}, webhook.ErrNotFound
}

func (m *mockWebhookRepo) CreateDelivery(_ context.Context, d webhook.WebhookDeliveryLog) (webhook.WebhookDeliveryLog, error) {
	d.ID = uuid.New()
	m.deliveries = append(m.deliveries, d)
	return d, nil
}

func (m *mockWebhookRepo) UpdateDelivery(_ context.Context, d webhook.WebhookDeliveryLog) (webhook.WebhookDeliveryLog, error) {
	for i, existing := range m.deliveries {
		if existing.ID == d.ID {
			m.deliveries[i] = d
			return d, nil
		}
	}
	return webhook.WebhookDeliveryLog{}, webhook.ErrNotFound
}

func TestWebhookService_Create_Success(t *testing.T) {
	svc := webhook.NewService(newMockRepo())
	w, err := svc.Create(context.Background(), webhook.CreateWebhookRequest{
		Name:   "Deploy Hook",
		URL:    "https://hooks.example.com/deploy",
		Events: []string{"agent.executed"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Deploy Hook", w.Name)
	assert.NotEqual(t, uuid.Nil, w.ID)
}

func TestWebhookService_GetByID_NotFound(t *testing.T) {
	svc := webhook.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, webhook.ErrNotFound)
}

func TestWebhookService_Delete_NotFound(t *testing.T) {
	svc := webhook.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, webhook.ErrNotFound)
}

func TestWebhookService_List(t *testing.T) {
	svc := webhook.NewService(newMockRepo())
	for _, name := range []string{"hook-1", "hook-2"} {
		_, err := svc.Create(context.Background(), webhook.CreateWebhookRequest{Name: name, URL: "https://x.com", Events: []string{"*"}})
		require.NoError(t, err)
	}
	items, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

// --- IngestWebhook tests ---

// newIngestWebhook creates a webhook with a secret and returns its token.
func newIngestWebhook(t *testing.T, svc *webhook.Service, targetURL string, events []string) string {
	t.Helper()
	secret := "test-secret-token"
	_, err := svc.Create(context.Background(), webhook.CreateWebhookRequest{
		Name:   "ingest-hook",
		URL:    targetURL,
		Events: events,
		Secret: &secret,
	})
	require.NoError(t, err)
	return secret
}

func TestIngestWebhook_GitHubSignatureValid(t *testing.T) {
	received := make(chan []byte, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received <- b
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := webhook.NewService(newMockRepo())
	secret := newIngestWebhook(t, svc, ts.URL, []string{"push"})

	payload := []byte(`{"ref":"refs/heads/main"}`)
	sig := "sha256=" + webhook.ComputeHMACSHA256(secret, payload)

	log, err := svc.IngestWebhook(context.Background(), secret, "github", payload, sig, "push")
	require.NoError(t, err)
	assert.Equal(t, "PENDING", log.Status)
	assert.Equal(t, "push", log.EventType)
}

func TestIngestWebhook_InvalidSignature_ReturnsError(t *testing.T) {
	svc := webhook.NewService(newMockRepo())
	secret := newIngestWebhook(t, svc, "http://localhost", []string{"push"})

	_, err := svc.IngestWebhook(context.Background(), secret, "github", []byte(`{}`), "sha256=badsig", "push")
	require.ErrorIs(t, err, webhook.ErrSignatureInvalid)
}

func TestIngestWebhook_EventFiltered(t *testing.T) {
	svc := webhook.NewService(newMockRepo())
	secret := newIngestWebhook(t, svc, "http://localhost", []string{"push"})
	sig := "sha256=" + webhook.ComputeHMACSHA256(secret, []byte(`{}`))

	_, err := svc.IngestWebhook(context.Background(), secret, "github", []byte(`{}`), sig, "pull_request")
	require.ErrorIs(t, err, webhook.ErrEventFiltered)
}

func TestIngestWebhook_TokenNotFound_ReturnsError(t *testing.T) {
	svc := webhook.NewService(newMockRepo())
	_, err := svc.IngestWebhook(context.Background(), "nonexistent-token", "github", []byte(`{}`), "", "push")
	require.ErrorIs(t, err, webhook.ErrNotFound)
}

func TestIngestWebhook_GitLabTokenValid(t *testing.T) {
	received := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := webhook.NewService(newMockRepo())
	secret := newIngestWebhook(t, svc, ts.URL, []string{"Push Hook"})

	log, err := svc.IngestWebhook(context.Background(), secret, "gitlab", []byte(`{}`), secret, "Push Hook")
	require.NoError(t, err)
	assert.Equal(t, "PENDING", log.Status)
}

func TestWebhookService_ListDeliveries_FilterByStatus(t *testing.T) {
	repo := newMockRepo()
	svc := webhook.NewService(repo)
	w := createWebhook(t, svc, nil)

	// Ingest two events with different results.
	d1, err := svc.Ingest(context.Background(), w.Token, "push", "", []byte("{}"))
	require.NoError(t, err)
	_, err = svc.RecordDeliveryResult(context.Background(), d1, 200, "OK", true)
	require.NoError(t, err)

	_, err = svc.Ingest(context.Background(), w.Token, "push", "", []byte("{}"))
	require.NoError(t, err)
	// Second delivery remains PENDING.

	pr := pagination.PageRequest{Page: 0, Size: 20}

	pageAll, err := svc.ListDeliveries(context.Background(), w.ID, webhook.DeliveryFilter{}, pr)
	require.NoError(t, err)
	assert.Equal(t, int64(2), pageAll.TotalElements)

	pageSuccess, err := svc.ListDeliveries(context.Background(), w.ID, webhook.DeliveryFilter{Status: webhook.DeliverySuccess}, pr)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pageSuccess.TotalElements)
	assert.Equal(t, webhook.DeliverySuccess, pageSuccess.Content[0].Status)

	pagePending, err := svc.ListDeliveries(context.Background(), w.ID, webhook.DeliveryFilter{Status: webhook.DeliveryPending}, pr)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pagePending.TotalElements)
}

func TestWebhookService_ListDeliveries_FilterByEventType(t *testing.T) {
	repo := newMockRepo()
	svc := webhook.NewService(repo)
	w := createWebhook(t, svc, nil)

	_, err := svc.Ingest(context.Background(), w.Token, "push", "", []byte("{}"))
	require.NoError(t, err)
	_, err = svc.Ingest(context.Background(), w.Token, "pull_request", "", []byte("{}"))
	require.NoError(t, err)

	pr := pagination.PageRequest{Page: 0, Size: 20}

	pagePush, err := svc.ListDeliveries(context.Background(), w.ID, webhook.DeliveryFilter{EventType: "push"}, pr)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pagePush.TotalElements)
	assert.Equal(t, "push", pagePush.Content[0].EventType)
}
