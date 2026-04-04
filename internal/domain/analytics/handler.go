package analytics

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// analyticsService defines the methods used by Handler.
type analyticsService interface {
	AgentUsage(ctx context.Context, agentID uuid.UUID, tr TimeRange) (UsageSummary, error)
	AgentCosts(ctx context.Context, agentID uuid.UUID, tr TimeRange) ([]CostSummary, error)
	TopTools(ctx context.Context, tr TimeRange, limit int) ([]TopTool, error)
}

// Handler exposes HTTP endpoints for analytics queries.
type Handler struct {
	svc analyticsService
}

// NewHandler creates a new analytics Handler.
func NewHandler(svc analyticsService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts analytics endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/analytics/agents/{agentId}/usage", h.agentUsage)
	r.Get("/api/analytics/agents/{agentId}/costs", h.agentCosts)
	r.Get("/api/analytics/tools/top", h.topTools)
}

func (h *Handler) agentUsage(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	tr, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.AgentUsage(r.Context(), agentID, tr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) agentCosts(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	tr, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.AgentCosts(r.Context(), agentID, tr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to query costs")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) topTools(w http.ResponseWriter, r *http.Request) {
	tr, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	result, err := h.svc.TopTools(r.Context(), tr, limit)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to query top tools")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

// parseTimeRange extracts from/to query parameters.
// Defaults to last 7 days if not provided.
func parseTimeRange(r *http.Request) (TimeRange, error) {
	now := time.Now()
	tr := TimeRange{
		From: now.AddDate(0, 0, -7),
		To:   now,
	}

	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return TimeRange{}, fmt.Errorf("invalid 'from' parameter: must be RFC3339")
		}
		tr.From = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return TimeRange{}, fmt.Errorf("invalid 'to' parameter: must be RFC3339")
		}
		tr.To = t
	}

	return tr, nil
}
