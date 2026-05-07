package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// metricsService is the interface required by Handler.
type metricsService interface {
	ListByAgent(ctx context.Context, tenantID string, agentID uuid.UUID, pr pagination.PageRequest) ([]AgentMetrics, int, error)
	Record(ctx context.Context, tenantID string, req RecordRequest) (AgentMetrics, error)
	GetAgentSummary(ctx context.Context, tenantID string, agentID uuid.UUID) (MetricsSummary, error)
	GetTenantSummary(ctx context.Context, tenantID string) (MetricsSummary, error)
	TopAgents(ctx context.Context, tenantID string, limit int) ([]AgentUsage, error)
	CostBreakdown(ctx context.Context, tenantID string) ([]CostBreakdownEntry, error)
}

// Handler handles HTTP requests for agent metrics.
type Handler struct {
	svc metricsService
}

// NewHandler creates a new Handler.
func NewHandler(svc metricsService) *Handler {
	return &Handler{svc: svc}
}

// AgentRoutes returns a router for agent-scoped metric endpoints. server.go
// already mounts this under /api/agents/{agentId}/metrics, so paths here
// must be relative to that prefix (was emitting /metrics/metrics before).
func (h *Handler) AgentRoutes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.listByAgent)
	r.Get("/summary", h.agentSummary)
	return r
}

// Routes returns a router for top-level metric endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/tenant", h.tenantSummary)
	r.Get("/top-agents", h.topAgents)
	r.Get("/cost-breakdown", h.costBreakdown)
	r.Post("/", h.record)
	return r
}

func (h *Handler) listByAgent(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	pr := pagination.ParsePageRequest(r)

	items, total, err := h.svc.ListByAgent(r.Context(), tenantID, agentID, pr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	dtos := make([]AgentMetricsResponse, len(items))
	for i, m := range items {
		dtos[i] = ResponseFrom(m)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

func (h *Handler) agentSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}

	s, err := h.svc.GetAgentSummary(r.Context(), tenantID, agentID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, s)
}

func (h *Handler) tenantSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	s, err := h.svc.GetTenantSummary(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, s)
}

func (h *Handler) topAgents(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	result, err := h.svc.TopAgents(r.Context(), tenantID, limit)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) costBreakdown(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	result, err := h.svc.CostBreakdown(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req RecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	m, err := h.svc.Record(r.Context(), tenantID, req)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, ResponseFrom(m))
}
