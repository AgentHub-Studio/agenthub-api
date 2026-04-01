package llmpreset

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Handler holds HTTP handlers for the LLM preset domain.
type Handler struct {
	svc Service
}

// NewHandler creates a new LLMPreset Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterProtectedRoutes mounts authenticated LLM preset routes onto r.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/api/llm-presets", h.list)
	r.Post("/api/llm-presets", h.create)
	r.Get("/api/llm-presets/by-provider/{provider}", h.listByProvider)
	r.Get("/api/llm-presets/{id}", h.get)
	r.Put("/api/llm-presets/{id}", h.update)
	r.Delete("/api/llm-presets/{id}", h.delete)
	r.Put("/api/llm-presets/{id}/default", h.setDefault)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.List(r.Context(), req)
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, page)
}

func (h *Handler) listByProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	req := pagination.ParsePageRequest(r)
	page, err := h.svc.ListByProvider(r.Context(), provider, req)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, page)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.BadRequest(w, "invalid preset id")
		return
	}
	p, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "preset not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, p)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateLLMPresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	p, err := h.svc.Create(r.Context(), req)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusCreated, p)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.BadRequest(w, "invalid preset id")
		return
	}
	var req UpdateLLMPresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.BadRequest(w, "invalid request body")
		return
	}
	p, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "preset not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	httputil.JSON(w, http.StatusOK, p)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.BadRequest(w, "invalid preset id")
		return
	}
	err = h.svc.Delete(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httputil.NotFound(w, "preset not found")
		return
	}
	if err != nil {
		httputil.InternalServerError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setDefault(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.BadRequest(w, "invalid preset id")
		return
	}
	if err := h.svc.SetDefault(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.NotFound(w, "preset not found")
			return
		}
		httputil.InternalServerError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
