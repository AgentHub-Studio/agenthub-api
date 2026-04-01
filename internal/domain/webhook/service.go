package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service handles business logic for webhooks.
type Service struct {
	repo WebhookRepository
}

// NewService creates a new webhook Service.
func NewService(repo WebhookRepository) *Service {
	return &Service{repo: repo}
}

// List returns all webhooks (secret omitted in responses).
func (s *Service) List(ctx context.Context) ([]WebhookConfig, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Secret = nil
	}
	return items, nil
}

// Create creates a new webhook configuration.
func (s *Service) Create(ctx context.Context, req CreateWebhookRequest) (WebhookConfig, error) {
	if req.Name == "" || req.URL == "" {
		return WebhookConfig{}, fmt.Errorf("webhook: name and url are required")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	retryCount := 3
	if req.RetryCount != nil {
		retryCount = *req.RetryCount
	}
	events := req.Events
	if events == nil {
		events = []string{}
	}
	w := WebhookConfig{
		Name:       req.Name,
		URL:        req.URL,
		Events:     events,
		Secret:     req.Secret,
		Enabled:    enabled,
		RetryCount: retryCount,
	}
	return s.repo.Create(ctx, w)
}

// GetByID returns a single webhook (secret omitted).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (WebhookConfig, error) {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return WebhookConfig{}, err
	}
	w.Secret = nil
	return w, nil
}

// Update updates a webhook configuration.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateWebhookRequest) (WebhookConfig, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return WebhookConfig{}, err
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.URL != nil {
		existing.URL = *req.URL
	}
	if req.Events != nil {
		existing.Events = req.Events
	}
	if req.Secret != nil {
		existing.Secret = req.Secret
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.RetryCount != nil {
		existing.RetryCount = *req.RetryCount
	}
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return WebhookConfig{}, err
	}
	updated.Secret = nil
	return updated, nil
}

// Delete removes a webhook configuration.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ListDeliveries returns paginated delivery logs.
func (s *Service) ListDeliveries(ctx context.Context, webhookID uuid.UUID, req pagination.PageRequest) (pagination.Page[WebhookDeliveryLog], error) {
	items, total, err := s.repo.ListDeliveries(ctx, webhookID, req)
	if err != nil {
		return pagination.Page[WebhookDeliveryLog]{}, err
	}
	return pagination.NewPage(items, total, req), nil
}

// SendTest creates a test delivery log entry with a sample payload.
func (s *Service) SendTest(ctx context.Context, webhookID uuid.UUID) (WebhookDeliveryLog, error) {
	_, err := s.repo.GetByID(ctx, webhookID)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}

	payload, _ := json.Marshal(map[string]any{
		"event":     "test",
		"timestamp": time.Now().UTC(),
		"webhookId": webhookID,
	})

	d := WebhookDeliveryLog{
		WebhookID: webhookID,
		EventType: "test",
		Payload:   payload,
		Status:    "PENDING",
		Attempts:  0,
	}
	return s.repo.CreateDelivery(ctx, d)
}
