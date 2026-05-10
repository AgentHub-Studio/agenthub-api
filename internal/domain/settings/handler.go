package settings

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
)

// ImpactAssessor counts agents that inherit the default provider and would be
// affected by a defaultProvider change.
type ImpactAssessor interface {
	CountPublishedWithoutProvider(ctx context.Context) (int64, error)
}

// ProviderImpactResponse describes how many agents would be affected by
// a change to the tenant-wide default LLM provider.
type ProviderImpactResponse struct {
	// AffectedAgents is the number of PUBLISHED agents that inherit the default
	// provider and would be re-routed to the new provider after the change.
	AffectedAgents int64 `json:"affectedAgents"`
}

// Handler holds HTTP handlers for the settings domain.
type Handler struct {
	svc      Service
	assessor ImpactAssessor // optional — nil disables the provider-impact endpoint
}

// NewHandler creates a new settings Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// WithImpactAssessor attaches an impact assessor to the handler, enabling
// GET /api/settings/provider-impact (ACT-F3-14).
func (h *Handler) WithImpactAssessor(a ImpactAssessor) *Handler {
	h.assessor = a
	return h
}

// RegisterProtectedRoutes mounts authenticated settings routes onto r.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/api/settings", h.list)
	r.Get("/api/settings/{key}", h.get)
	r.Put("/api/settings/{key}", h.upsert)
	r.Delete("/api/settings/{key}", h.delete)

	// Provider model listing endpoints.
	r.Get("/api/settings/providers", h.listProviders)
	r.Get("/api/settings/openai/models", h.listOpenAIModels)
	r.Get("/api/settings/anthropic/models", h.listAnthropicModels)
	r.Get("/api/settings/ollama/models", h.listOllamaModels)
	r.Get("/api/settings/openrouter/models", h.listOpenRouterModels)
	r.Get("/api/settings/openrouter/embedding-models", h.listOpenRouterEmbeddingModels)
	r.Post("/api/settings/smtp/test", h.testSmtp)

	// Provider-impact preview (ACT-F3-14 / P-C337-1).
	r.Get("/api/settings/provider-impact", h.providerImpact)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.List(r.Context())
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, settings)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	s, err := h.svc.Get(r.Context(), key)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "setting not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, s)
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req UpdateSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	s, err := h.svc.Upsert(r.Context(), key, req)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			httputil.UnprocessableEntity(w, err.Error())
			return
		}
		// Bug 257: fallback após ErrValidation só vê repo SQL errors.
		// Não vazar SQLSTATE/pgx detail para o cliente.
		slog.Error("settings: upsert failed", "key", key, "err", err)
		httputil.InternalServerError(w, "upsert failed")
		return
	}
	httputil.JSON(w, http.StatusOK, s)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	err := h.svc.Delete(r.Context(), key)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "setting not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// providerImpact returns how many PUBLISHED agents would be affected by a
// change to the tenant-wide defaultProvider setting.
func (h *Handler) providerImpact(w http.ResponseWriter, r *http.Request) {
	if h.assessor == nil {
		httputil.JSON(w, http.StatusOK, ProviderImpactResponse{AffectedAgents: 0})
		return
	}
	count, err := h.assessor.CountPublishedWithoutProvider(r.Context())
	if err != nil {
		httputil.InternalServerError(w, "failed to assess provider impact")
		return
	}
	httputil.JSON(w, http.StatusOK, ProviderImpactResponse{AffectedAgents: count})
}

func (h *Handler) listProviders(w http.ResponseWriter, _ *http.Request) {
	httputil.JSON(w, http.StatusOK, ListProviders())
}

func (h *Handler) listOpenAIModels(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-OpenAI-API-Key")
	if apiKey == "" {
		httputil.BadRequest(w, "X-OpenAI-API-Key header is required")
		return
	}
	models, err := ListOpenAIModels(r.Context(), apiKey)
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, models)
}

func (h *Handler) listAnthropicModels(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-Claude-API-Key")
	models, err := ListAnthropicModels(r.Context(), apiKey)
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, models)
}

func (h *Handler) listOllamaModels(w http.ResponseWriter, r *http.Request) {
	baseURL := r.URL.Query().Get("baseUrl")
	models, err := ListOllamaModels(r.Context(), baseURL)
	if err != nil {
		if errors.Is(err, ErrUpstream) {
			httputil.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, models)
}

func (h *Handler) listOpenRouterModels(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-OpenRouter-API-Key")
	models, err := ListOpenRouterModels(r.Context(), apiKey)
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, models)
}

func (h *Handler) listOpenRouterEmbeddingModels(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-OpenRouter-API-Key")
	models, err := ListOpenRouterEmbeddingModels(r.Context(), apiKey)
	if err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	httputil.JSON(w, http.StatusOK, models)
}

func (h *Handler) testSmtp(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	if to == "" {
		httputil.BadRequest(w, "to query parameter is required")
		return
	}
	var req SmtpTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	if err := TestSMTPConnection(r.Context(), req); err != nil {
		httputil.InternalServerError(w, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
