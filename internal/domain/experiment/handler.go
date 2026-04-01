package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// experimentService is the interface required by Handler.
type experimentService interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]PromptExperiment, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error)
	Create(ctx context.Context, tenantID string, req CreateRequest) (PromptExperiment, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (PromptExperiment, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
	Activate(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error)
	Pause(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error)
	Complete(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error)
	RecordResult(ctx context.Context, tenantID string, experimentID uuid.UUID, req RecordResultRequest) (ExperimentResult, error)
	GetResults(ctx context.Context, tenantID string, experimentID uuid.UUID, pr pagination.PageRequest) ([]ExperimentResult, int, error)
	SelectVariant(ctx context.Context, tenantID string, id uuid.UUID, sessionID string) (string, error)
	GetSummary(ctx context.Context, tenantID string, id uuid.UUID) (ExperimentSummary, error)
}

// Handler handles HTTP requests for prompt experiments.
type Handler struct {
	svc experimentService
}

// NewHandler creates a new Handler.
func NewHandler(svc experimentService) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the handler routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/activate", h.activate)
	r.Post("/{id}/pause", h.pause)
	r.Post("/{id}/complete", h.complete)
	r.Post("/{id}/results", h.recordResult)
	r.Get("/{id}/results", h.getResults)
	r.Get("/{id}/summary", h.getSummary)
	r.Get("/{id}/select-variant", h.selectVariant)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)

	items, total, err := h.svc.ListAll(r.Context(), tenantID, pr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	dtos := make([]PromptExperimentResponse, len(items))
	for i, e := range items {
		dtos[i] = ResponseFrom(e)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.svc.Create(r.Context(), tenantID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, ResponseFrom(created))
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	e, err := h.svc.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(e))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.svc.Update(r.Context(), tenantID, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(updated))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Delete(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.NoContent(w)
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	h.transitionStatus(w, r, h.svc.Activate)
}

func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	h.transitionStatus(w, r, h.svc.Pause)
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	h.transitionStatus(w, r, h.svc.Complete)
}

func (h *Handler) transitionStatus(w http.ResponseWriter, r *http.Request, fn func(context.Context, string, uuid.UUID) (PromptExperiment, error)) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	e, err := fn(r.Context(), tenantID, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			respond.Error(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrInvalidTransition):
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		default:
			respond.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(e))
}

func (h *Handler) recordResult(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req RecordResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.svc.RecordResult(r.Context(), tenantID, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, ResultResponseFrom(res))
}

func (h *Handler) getResults(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	pr := pagination.ParsePageRequest(r)

	items, total, err := h.svc.GetResults(r.Context(), tenantID, id, pr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	dtos := make([]ExperimentResultResponse, len(items))
	for i, res := range items {
		dtos[i] = ResultResponseFrom(res)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

// getSummary returns aggregated variant metrics for an experiment.
func (h *Handler) getSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	summary, err := h.svc.GetSummary(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, summary)
}

// selectVariant deterministically picks a variant for the given session_id query param.
func (h *Handler) selectVariant(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		respond.Error(w, http.StatusBadRequest, "sessionId query parameter is required")
		return
	}
	variant, err := h.svc.SelectVariant(r.Context(), tenantID, id, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"variantKey": variant})
}
