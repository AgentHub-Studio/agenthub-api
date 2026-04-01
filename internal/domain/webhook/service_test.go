package webhook_test

import (
	"context"
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

func (m *mockWebhookRepo) ListDeliveries(_ context.Context, webhookID uuid.UUID, _ pagination.PageRequest) ([]webhook.WebhookDeliveryLog, int64, error) {
	var out []webhook.WebhookDeliveryLog
	for _, d := range m.deliveries {
		if d.WebhookID == webhookID {
			out = append(out, d)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockWebhookRepo) CreateDelivery(_ context.Context, d webhook.WebhookDeliveryLog) (webhook.WebhookDeliveryLog, error) {
	d.ID = uuid.New()
	m.deliveries = append(m.deliveries, d)
	return d, nil
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
