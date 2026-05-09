package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// webhookService defines the methods used by Handler.
type webhookService interface {
	List(ctx context.Context) ([]WebhookConfig, error)
	Create(ctx context.Context, req CreateWebhookRequest) (WebhookConfig, error)
	GetByID(ctx context.Context, id uuid.UUID) (WebhookConfig, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateWebhookRequest) (WebhookConfig, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListDeliveries(ctx context.Context, webhookID uuid.UUID, filter DeliveryFilter, req pagination.PageRequest) (pagination.Page[WebhookDeliveryLog], error)
	IngestWebhook(ctx context.Context, token, sourceType string, payload []byte, signature, eventType string) (WebhookDeliveryLog, error)
	SendTest(ctx context.Context, webhookID uuid.UUID) (WebhookDeliveryLog, error)
}

// Handler exposes webhook HTTP endpoints.
type Handler struct {
	svc webhookService
}

// NewHandler creates a new Handler.
func NewHandler(svc webhookService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts webhook routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/webhooks", h.list)
	r.Post("/api/webhooks", h.create)
	r.Get("/api/webhooks/{id}", h.getByID)
	r.Put("/api/webhooks/{id}", h.update)
	r.Patch("/api/webhooks/{id}", h.update)
	r.Delete("/api/webhooks/{id}", h.delete)
	r.Get("/api/webhooks/{id}/deliveries", h.listDeliveries)
	r.Post("/api/webhooks/{id}/test", h.sendTest)
}

// RegisterPublicRoutes mounts public webhook routes (no auth required).
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/api/webhooks/{token}/ingest", h.ingest)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		// Bug 195: nunca expor err.Error() em fallback 500.
		slog.Error("webhook: list failed", "err", err)
		respond.Error(w, http.StatusInternalServerError, "list failed")
		return
	}
	respond.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateName) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		slog.Error("webhook: getByID failed", "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "get failed")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.NoContent(w)
}

func (h *Handler) listDeliveries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	// Bug 211: valida webhook antes de listar deliveries (evita 200+empty
	// para webhook inexistente — UX e probing consistency).
	if _, err := h.svc.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	req := pagination.ParsePageRequest(r)
	filter := DeliveryFilter{
		Status:    r.URL.Query().Get("status"),
		EventType: r.URL.Query().Get("eventType"),
	}
	page, err := h.svc.ListDeliveries(r.Context(), id, filter, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

// ingest handles POST /api/webhooks/{token}/ingest.
// It is a public endpoint — authentication is provided by the token itself.
func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		respond.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Detect source type from request headers.
	sourceType := "github"
	signature := r.Header.Get("X-Hub-Signature-256")
	eventType := r.Header.Get("X-GitHub-Event")
	if signature == "" {
		sourceType = "gitlab"
		signature = r.Header.Get("X-Gitlab-Token")
		eventType = r.Header.Get("X-Gitlab-Event")
	}
	if eventType == "" {
		eventType = "push"
	}

	log, err := h.svc.IngestWebhook(r.Context(), token, sourceType, body, signature, eventType)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		if errors.Is(err, ErrSignatureInvalid) {
			respond.Error(w, http.StatusUnauthorized, "invalid signature")
			return
		}
		if errors.Is(err, ErrEventFiltered) {
			respond.JSON(w, http.StatusOK, map[string]string{"status": "filtered"})
			return
		}
		// Bug 190: endpoint público — repo error (não-42P01) ou falha em
		// CreateDelivery não pode vazar SQLSTATE/wrappers. Log completo;
		// resposta opaca preserva confidencialidade.
		slog.Error("webhook: ingest internal failure", "sourceType", sourceType, "err", err)
		respond.Error(w, http.StatusInternalServerError, "ingest failed")
		return
	}
	respond.JSON(w, http.StatusAccepted, log)
}

func (h *Handler) sendTest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	resp, err := h.svc.SendTest(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}
