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
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrSignatureInvalid is returned when a webhook signature does not match.
var ErrSignatureInvalid = fmt.Errorf("webhook: invalid signature")

// ErrEventFiltered is returned when the event type is not in the webhook's allowed list.
var ErrEventFiltered = fmt.Errorf("webhook: event filtered")

const (
	ingestBaseDelay  = time.Second
	ingestMaxRetries = 5
)

// TenantLister enumerates tenant IDs for cross-tenant token lookup
// (bug 241). Public ingest endpoint não tem JWT/tenant context, então
// precisa varrer todos os schemas ah_* até achar o webhook pelo token.
type TenantLister interface {
	ListAllIDs(ctx context.Context) ([]string, error)
}

// Service handles business logic for webhooks.
type Service struct {
	repo         WebhookRepository
	tenantLister TenantLister
}

// NewService creates a new webhook Service.
func NewService(repo WebhookRepository) *Service {
	return &Service{repo: repo}
}

// WithTenantLister wires a tenant lister enabling cross-tenant token
// lookup at the public ingest endpoint (bug 241).
func (s *Service) WithTenantLister(t TenantLister) *Service {
	s.tenantLister = t
	return s
}

// lookupConfigByToken finds the webhook that owns the given token,
// iterating tenants when ctx has no tenant (public endpoint case).
func (s *Service) lookupConfigByToken(ctx context.Context, token string) (WebhookConfig, string, error) {
	if tid := tenantctx.FromContext(ctx); tid != "" {
		if w, err := s.repo.GetByToken(ctx, token); err == nil {
			return w, tid, nil
		}
	} else if w, err := s.repo.GetByToken(ctx, token); err == nil {
		return w, "", nil
	}
	if s.tenantLister == nil {
		return WebhookConfig{}, "", ErrNotFound
	}
	tids, err := s.tenantLister.ListAllIDs(ctx)
	if err != nil {
		return WebhookConfig{}, "", fmt.Errorf("webhook: list tenants: %w", err)
	}
	for _, tid := range tids {
		tctx := tenantctx.NewContext(ctx, tid)
		if w, err := s.repo.GetByToken(tctx, token); err == nil {
			return w, tid, nil
		}
	}
	return WebhookConfig{}, "", ErrNotFound
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
	// Bug 181: strip HTML do name (XSS prevention).
	req.Name = sanitize.StripHTML(req.Name)
	if req.Name == "" || req.URL == "" {
		return WebhookConfig{}, fmt.Errorf("%w: name and url are required", ErrValidation)
	}
	// Bug 129: name varchar(255) — gate length antes do INSERT
	// (era 422 mas com SQL error 22001 vazando para o cliente).
	if len(req.Name) > 255 {
		return WebhookConfig{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	// Bug 144: events filter — webhook sem eventos nunca dispara (filtro
	// nunca casa). Permitir era criar entry zumbi no banco.
	if len(req.Events) == 0 {
		return WebhookConfig{}, fmt.Errorf("%w: events must contain at least one event type", ErrValidation)
	}
	// Bug 165: cap events count em 50. Webhook real escuta poucos eventos;
	// 1000+ é payload malformado.
	if len(req.Events) > 50 {
		return WebhookConfig{}, fmt.Errorf("%w: events list exceeds maximum of 50 entries (got %d)", ErrValidation, len(req.Events))
	}
	for _, e := range req.Events {
		if strings.TrimSpace(e) == "" {
			return WebhookConfig{}, fmt.Errorf("%w: events must not contain empty strings", ErrValidation)
		}
		if len(e) > 100 {
			return WebhookConfig{}, fmt.Errorf("%w: event name exceeds maximum length of 100 chars (got %d)", ErrValidation, len(e))
		}
	}
	// Bug 164: cap URL em 2048 chars (RFC standard; webhook URLs reais
	// não passam disso). Sem isso, 200KB+ silenciosamente aceito.
	if len(req.URL) > 2048 {
		return WebhookConfig{}, fmt.Errorf("%w: url exceeds maximum length of 2048 chars (got %d)", ErrValidation, len(req.URL))
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
	created, err := s.repo.Create(ctx, w)
	if err != nil {
		return WebhookConfig{}, err
	}
	// Bug 154: secret é write-only — usado para HMAC signature mas nunca
	// retornado. List/GetByID já mascaram; Create esquecia, vazando para
	// quem cria o webhook (ainda assim é leak — frontend logging, request
	// inspect, etc.).
	created.Secret = nil
	return created, nil
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
	if req.Name != nil && len(*req.Name) > 255 {
		return WebhookConfig{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(*req.Name))
	}
	if req.Name != nil {
		// Bug 181: strip HTML (XSS prevention).
		stripped := sanitize.StripHTML(*req.Name)
		req.Name = &stripped
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
		// Bug 145: mesmo gate do Create — events não pode ser vazio
		// nem conter strings vazias. Sem isso, UPDATE cria webhook
		// zumbi a partir de webhook válido.
		if len(req.Events) == 0 {
			return WebhookConfig{}, fmt.Errorf("%w: events must contain at least one event type", ErrValidation)
		}
		// Bug 172: cap em Update (cross-cutting com Create — bug 165).
		if len(req.Events) > 50 {
			return WebhookConfig{}, fmt.Errorf("%w: events list exceeds maximum of 50 entries (got %d)", ErrValidation, len(req.Events))
		}
		for _, e := range req.Events {
			if strings.TrimSpace(e) == "" {
				return WebhookConfig{}, fmt.Errorf("%w: events must not contain empty strings", ErrValidation)
			}
			if len(e) > 100 {
				return WebhookConfig{}, fmt.Errorf("%w: event name exceeds maximum length of 100 chars (got %d)", ErrValidation, len(e))
			}
		}
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
	// Bug 241: lookup via column `token` (URL param é o token público,
	// não o secret HMAC) e itera tenants quando ctx é público.
	// Fallback para legacy GetBySecret se não houver match (compat com
	// webhooks antigos pré-migration que usavam secret como token).
	w, resolvedTenant, err := s.lookupConfigByToken(ctx, token)
	if err != nil {
		w, err = s.repo.GetBySecret(ctx, token)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
				return WebhookDeliveryLog{}, ErrNotFound
			}
			if errors.Is(err, ErrNotFound) {
				return WebhookDeliveryLog{}, ErrNotFound
			}
			return WebhookDeliveryLog{}, err
		}
	}
	// Propaga tenant resolvido para downstream (CreateDelivery + dispatch).
	if resolvedTenant != "" {
		ctx = tenantctx.NewContext(ctx, resolvedTenant)
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

	// Dispatch asynchronously with retry. Bug 236: use the context-aware
	// variant to preserve tenant values. Without this, UpdateDelivery uses the
	// wrong search_path and the delivery remains PENDING.
	go s.dispatchWithRetryCtx(ctx, w, log)

	return log, nil
}

// dispatchWithRetryCtx mirrors dispatchWithRetry but preserves request context
// values, including tenant, while detaching from request cancellation.
func (s *Service) dispatchWithRetryCtx(parentCtx context.Context, w WebhookConfig, d WebhookDeliveryLog) {
	maxAttempts := w.RetryCount
	if maxAttempts <= 0 || maxAttempts > ingestMaxRetries {
		maxAttempts = ingestMaxRetries
	}

	ctx := detachedContext(parentCtx)
	if tid := tenantctx.FromContext(parentCtx); tid != "" && tenantctx.FromContext(ctx) == "" {
		ctx = tenantctx.NewContext(ctx, tid)
	}
	client := newWebhookHTTPClient()

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

func detachedContext(parent context.Context) context.Context {
	if parent == nil {
		return context.Background()
	}
	return context.WithoutCancel(parent)
}

func newWebhookHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(redirect *http.Request, _ []*http.Request) error {
			if err := ssrf.ValidateURL(redirect.URL.String()); err != nil {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// doHTTPPost sends a POST request with the given payload and returns status code, body, error.
func doHTTPPost(client *http.Client, url string, payload []byte) (int, string, error) {
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
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
	w, resolvedTenant, err := s.lookupConfigByToken(ctx, token)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}
	if resolvedTenant != "" {
		ctx = tenantctx.NewContext(ctx, resolvedTenant)
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

	// Bug 236: preserve tenant context for the goroutine.
	go s.dispatchWithRetryCtx(ctx, w, log)
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
	w, err := s.repo.GetByID(ctx, webhookID)
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
	log, err := s.repo.CreateDelivery(ctx, d)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}
	// Bug 236: SendTest used to create a PENDING delivery without dispatching
	// it. Keep the same context-preserving dispatch pattern used by Ingest.
	go s.dispatchWithRetryCtx(ctx, w, log)
	return log, nil
}

// DispatchEvent sends an internal AgentHub event to an existing outgoing
// webhook configuration, recording the attempt in webhook_delivery_log.
func (s *Service) DispatchEvent(ctx context.Context, webhookID uuid.UUID, eventType string, payload any) (WebhookDeliveryLog, error) {
	w, err := s.repo.GetByID(ctx, webhookID)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}
	if !w.Enabled {
		return WebhookDeliveryLog{}, ErrEventFiltered
	}
	if len(w.Events) > 0 && !containsEvent(w.Events, eventType) {
		return WebhookDeliveryLog{}, ErrEventFiltered
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return WebhookDeliveryLog{}, fmt.Errorf("webhook: marshal dispatch payload: %w", err)
	}
	d := WebhookDeliveryLog{
		WebhookID: webhookID,
		EventType: eventType,
		Payload:   raw,
		Status:    DeliveryPending,
		Attempts:  0,
	}
	log, err := s.repo.CreateDelivery(ctx, d)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}
	go s.dispatchWithRetryCtx(ctx, w, log)
	return log, nil
}
