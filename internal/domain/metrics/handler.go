package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
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

// agentExister verifies parent-agent existence (bug 222 batch).
type agentExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// Handler handles HTTP requests for agent metrics.
type Handler struct {
	svc   metricsService
	agent agentExister
}

// NewHandler creates a new Handler.
func NewHandler(svc metricsService) *Handler {
	return &Handler{svc: svc}
}

// WithAgentExister wires the parent-agent existence checker (bug 222).
func (h *Handler) WithAgentExister(a agentExister) *Handler {
	h.agent = a
	return h
}

// AgentRoutes returns a router for agent-scoped metric endpoints. server.go
// already mounts this under /api/agents/{id}/metrics, so paths here
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
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	if h.agent != nil {
		if err := h.agent.GetByID(r.Context(), agentID); err != nil {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
	}
	pr := pagination.ParsePageRequest(r)

	items, total, err := h.svc.ListByAgent(r.Context(), tenantID, agentID, pr)
	if err != nil {
		// Bug 193: nunca expor err.Error() em fallback 500.
		slog.Error("metrics: listByAgent failed", "tenantID", tenantID, "agentID", agentID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "list failed")
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
	agentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid agentId")
		return
	}
	if h.agent != nil {
		if err := h.agent.GetByID(r.Context(), agentID); err != nil {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
	}

	s, err := h.svc.GetAgentSummary(r.Context(), tenantID, agentID)
	if err != nil {
		slog.Error("metrics: agentSummary failed", "tenantID", tenantID, "agentID", agentID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "summary failed")
		return
	}
	respond.JSON(w, http.StatusOK, s)
}

func (h *Handler) tenantSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	s, err := h.svc.GetTenantSummary(r.Context(), tenantID)
	if err != nil {
		slog.Error("metrics: tenantSummary failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "summary failed")
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
		slog.Error("metrics: topAgents failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "top agents failed")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) costBreakdown(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	result, err := h.svc.CostBreakdown(r.Context(), tenantID)
	if err != nil {
		slog.Error("metrics: costBreakdown failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "cost breakdown failed")
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req RecordRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SessionID != "" {
		if _, err := uuid.Parse(req.SessionID); err != nil {
			respond.Error(w, http.StatusUnprocessableEntity, "invalid sessionId")
			return
		}
	}

	m, err := h.svc.Record(r.Context(), tenantID, req)
	if err != nil {
		slog.Error("metrics: record failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "record failed")
		return
	}
	respond.JSON(w, http.StatusCreated, ResponseFrom(m))
}
