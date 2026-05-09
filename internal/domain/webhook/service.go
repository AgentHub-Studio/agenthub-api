package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

// ErrSignatureInvalid is returned when a webhook signature does not match.
var ErrSignatureInvalid = fmt.Errorf("webhook: invalid signature")

// ErrEventFiltered is returned when the event type is not in the webhook's allowed list.
var ErrEventFiltered = fmt.Errorf("webhook: event filtered")

const (
	ingestBaseDelay = time.Second
	ingestMaxRetries = 5
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
	if items == nil {
		items = []WebhookConfig{}
	}
	for i := range items {
		items[i].Secret = nil
	}
	return items, nil
}

// Create creates a new webhook configuration.
func (s *Service) Create(ctx context.Context, req CreateWebhookRequest) (WebhookConfig, error) {
	if req.Name == "" || req.URL == "" {
		return WebhookConfig{}, fmt.Errorf("%w: name and url are required", ErrValidation)
	}
	if u, err := url.Parse(req.URL); err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return WebhookConfig{}, fmt.Errorf("%w: url must be a valid http(s) URL with host", ErrValidation)
	}
	// Bug 99: prevent SSRF — webhooks must not target localhost, private IPs,
	// or cloud metadata endpoints. Webhook fires from inside the cluster, so
	// allowing them would let users probe internal services or steal IAM creds.
	if err := ssrf.ValidateURL(req.URL); err != nil {
		return WebhookConfig{}, fmt.Errorf("%w: invalid URL (%v)", ErrValidation, err)
	}
	exists, err := s.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return WebhookConfig{}, fmt.Errorf("webhook: check duplicate name: %w", err)
	}
	if exists {
		return WebhookConfig{}, ErrDuplicateName
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	retryCount := 3
	if req.RetryCount != nil {
		if *req.RetryCount < 0 || *req.RetryCount > 10 {
			return WebhookConfig{}, fmt.Errorf("%w: retryCount must be between 0 and 10 (got %d)", ErrValidation, *req.RetryCount)
		}
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
		Token:      uuid.New().String(),
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
		// Bug 115: name vazio salvo na DB causa label vazio na UI.
		// Create rejeita; Update precisa do mesmo gate.
		if *req.Name == "" {
			return WebhookConfig{}, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		existing.Name = *req.Name
	}
	if req.URL != nil {
		// Bug 104: Update precisa do mesmo gate SSRF que Create —
		// senão admin malicioso cria webhook benigno e depois PATCH
		// para http://localhost ou 169.254.169.254.
		if err := ssrf.ValidateURL(*req.URL); err != nil {
			return WebhookConfig{}, fmt.Errorf("%w: invalid URL (%v)", ErrValidation, err)
		}
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
		if *req.RetryCount < 0 || *req.RetryCount > 10 {
			return WebhookConfig{}, fmt.Errorf("%w: retryCount must be between 0 and 10 (got %d)", ErrValidation, *req.RetryCount)
		}
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

// ListDeliveries returns paginated delivery logs with optional status/event_type filters.
func (s *Service) ListDeliveries(ctx context.Context, webhookID uuid.UUID, filter DeliveryFilter, req pagination.PageRequest) (pagination.Page[WebhookDeliveryLog], error) {
	items, total, err := s.repo.ListDeliveries(ctx, webhookID, filter, req)
	if err != nil {
		return pagination.Page[WebhookDeliveryLog]{}, err
	}
	return pagination.NewPage(items, total, req), nil
}

// IngestWebhook receives an event from an external system (e.g. GitHub, GitLab),
// validates the signature, filters by event type, creates a delivery log, and
// asynchronously dispatches the payload to the webhook URL with exponential-backoff retry.
//
// sourceType must be "github" or "gitlab".
// signature is the raw value of X-Hub-Signature-256 (GitHub) or X-Gitlab-Token (GitLab).
func (s *Service) IngestWebhook(ctx context.Context, token, sourceType string, payload []byte, signature, eventType string) (WebhookDeliveryLog, error) {
	w, err := s.repo.GetBySecret(ctx, token)
	if err != nil {
		// Endpoint público (sem JWT/tenant context) → search_path é
		// public, mas webhook_config só existe em schemas ah_*. Mapeia
		// "relation does not exist" para ErrNotFound (404) em vez de
		// vazar 500. Backlog #198: cross-schema lookup ou tabela global.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return WebhookDeliveryLog{}, ErrNotFound
		}
		return WebhookDeliveryLog{}, err
	}

	// Validate signature.
	if w.Secret != nil && *w.Secret != "" {
		if err := validateSignature(sourceType, *w.Secret, payload, signature); err != nil {
			return WebhookDeliveryLog{}, err
		}
	}

	// Filter event type.
	if len(w.Events) > 0 && !containsEvent(w.Events, eventType) {
		return WebhookDeliveryLog{}, ErrEventFiltered
	}

	// Create delivery log.
	d := WebhookDeliveryLog{
		WebhookID: w.ID,
		EventType: eventType,
		Payload:   payload,
		Status:    "PENDING",
		Attempts:  0,
	}
	log, err := s.repo.CreateDelivery(ctx, d)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}

	// Dispatch asynchronously with retry.
	go s.dispatchWithRetry(w, log)

	return log, nil
}

// dispatchWithRetry forwards the payload to the webhook's target URL with exponential backoff.
// Delays: 1s, 2s, 4s, 8s, 16s (2^attempt * baseDelay, capped at ingestMaxRetries attempts).
func (s *Service) dispatchWithRetry(w WebhookConfig, d WebhookDeliveryLog) {
	maxAttempts := w.RetryCount
	if maxAttempts <= 0 || maxAttempts > ingestMaxRetries {
		maxAttempts = ingestMaxRetries
	}

	ctx := context.Background()
	client := &http.Client{Timeout: 10 * time.Second}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * ingestBaseDelay
			time.Sleep(delay)
		}

		d.Attempts = attempt + 1
		status, body, err := doHTTPPost(client, w.URL, d.Payload)
		if err == nil && status >= 200 && status < 300 {
			now := time.Now().UTC()
			d.Status = "DELIVERED"
			d.ResponseStatus = &status
			d.ResponseBody = &body
			d.DeliveredAt = &now
			_, _ = s.repo.UpdateDelivery(ctx, d)
			return
		}
		if err == nil {
			d.ResponseStatus = &status
			d.ResponseBody = &body
		}
	}

	d.Status = "FAILED"
	_, _ = s.repo.UpdateDelivery(ctx, d)
}

// doHTTPPost sends a POST request with the given payload and returns status code, body, error.
func doHTTPPost(client *http.Client, url string, payload []byte) (int, string, error) {
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(b), nil
}

// validateSignature checks the HMAC-SHA256 signature for GitHub or token equality for GitLab.
func validateSignature(sourceType, secret string, payload []byte, signature string) error {
	switch strings.ToLower(sourceType) {
	case "github":
		// GitHub sends: sha256=<hex>
		expected := "sha256=" + ComputeHMACSHA256(secret, payload)
		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return ErrSignatureInvalid
		}
	case "gitlab":
		// GitLab sends the secret token directly.
		if signature != secret {
			return ErrSignatureInvalid
		}
	}
	return nil
}

// ComputeHMACSHA256 returns the hex-encoded HMAC-SHA256 of payload using key.
// Exported for use in tests.
func ComputeHMACSHA256(key string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// containsEvent returns true if event is in the allowed list or the list contains "*".
func containsEvent(allowed []string, event string) bool {
	for _, e := range allowed {
		if e == "*" || e == event {
			return true
		}
	}
	return false
}

// Ingest receives a generic webhook event, authenticates by token, creates a delivery
// log and asynchronously dispatches to the configured URL. This is a simplified
// alternative to IngestWebhook for non-signed payloads (internal or testing use).
func (s *Service) Ingest(ctx context.Context, token, eventType, signature string, payload []byte) (WebhookDeliveryLog, error) {
	w, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}

	if len(w.Events) > 0 && !containsEvent(w.Events, eventType) {
		return WebhookDeliveryLog{}, ErrEventFiltered
	}

	d := WebhookDeliveryLog{
		WebhookID: w.ID,
		EventType: eventType,
		Payload:   payload,
		Status:    DeliveryPending,
		Attempts:  0,
	}
	log, err := s.repo.CreateDelivery(ctx, d)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}

	go s.dispatchWithRetry(w, log)
	return log, nil
}

// RecordDeliveryResult updates a delivery log with the HTTP response from the target.
func (s *Service) RecordDeliveryResult(ctx context.Context, d WebhookDeliveryLog, statusCode int, responseBody string, success bool) (WebhookDeliveryLog, error) {
	d.ResponseStatus = &statusCode
	d.ResponseBody = &responseBody
	if success {
		d.Status = DeliverySuccess
	} else {
		d.Status = DeliveryFailed
	}
	return s.repo.UpdateDelivery(ctx, d)
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
