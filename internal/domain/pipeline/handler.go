// Package pipeline provides read-only deprecated endpoints for the pipeline (DAG) resource.
// Pipelines have been superseded by the agentic runner (ADR-012) and are deprecated since
// 2026-04-02 with a sunset date of 2026-07-01. These endpoints are retained only for
// backward compatibility and must not support writes.
package pipeline

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

const (
	deprecationDate = "2026-04-02"
	sunsetDate      = "2026-07-01"
	deprecationLink = "https://docs.agenthub.dev/migration/pipelines-to-agentic"
)

// PipelineResponse is the read-only DTO returned by deprecated endpoints.
type PipelineResponse struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	AgentID     *uuid.UUID `json:"agentId"`
	Status      string     `json:"status"`
	Config      any        `json:"config"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// pipelineRepository is the minimal read-only interface used by Handler.
type pipelineRepository interface {
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[PipelineResponse], error)
	GetByID(ctx context.Context, id uuid.UUID) (PipelineResponse, error)
}

// Handler handles deprecated read-only pipeline HTTP requests.
type Handler struct {
	repo pipelineRepository
}

// NewHandler creates a new pipeline Handler.
func NewHandler(repo pipelineRepository) *Handler {
	return &Handler{repo: repo}
}

// RegisterRoutes mounts the deprecated read-only pipeline routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(deprecatedHeaders).Get("/api/pipelines", h.list)
	r.With(deprecatedHeaders).Get("/api/pipelines/{id}", h.getByID)
}

// deprecatedHeaders injects RFC 8594 Deprecation and Sunset headers on every response.
func deprecatedHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", deprecationDate)
		w.Header().Set("Sunset", sunsetDate)
		w.Header().Set("Link", `<`+deprecationLink+`>; rel="deprecation"`)
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := h.repo.List(r.Context(), pagination.ParsePageRequest(r))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list pipelines")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	p, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err.Error() == "not found" {
			respond.Error(w, http.StatusNotFound, "pipeline not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get pipeline")
		return
	}
	respond.JSON(w, http.StatusOK, p)
}
