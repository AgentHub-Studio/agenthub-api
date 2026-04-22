package probe

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// Handler exposes integration connection-test endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler wrapping the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the /test endpoints on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/integrations/http/test", h.testHTTP)
	r.Post("/api/integrations/database/test", h.testDatabase)
	r.Post("/api/integrations/mcp/test", h.testMCP)
}

func (h *Handler) testHTTP(w http.ResponseWriter, r *http.Request) {
	var req HTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respond.JSON(w, http.StatusOK, h.svc.HTTP(r.Context(), req))
}

func (h *Handler) testDatabase(w http.ResponseWriter, r *http.Request) {
	var req DatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respond.JSON(w, http.StatusOK, h.svc.Database(r.Context(), req))
}

func (h *Handler) testMCP(w http.ResponseWriter, r *http.Request) {
	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respond.JSON(w, http.StatusOK, h.svc.MCP(r.Context(), req))
}
