package agent

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// BundleHandler exposes bundle export/import HTTP endpoints.
type BundleHandler struct {
	exporter *Exporter
	importer *Importer
}

// NewBundleHandler creates a BundleHandler.
func NewBundleHandler(exporter *Exporter, importer *Importer) *BundleHandler {
	return &BundleHandler{exporter: exporter, importer: importer}
}

// RegisterBundleRoutes mounts bundle routes on the given router.
func (h *BundleHandler) RegisterBundleRoutes(r chi.Router) {
	r.Get("/api/agents/{id}/export", h.exportBundle)
	r.Post("/api/agents/import", h.importBundle)
}

// exportBundle handles GET /api/agents/{id}/export
// Returns the agent and its skills as a portable JSON bundle.
func (h *BundleHandler) exportBundle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	bundle, err := h.exporter.Export(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "agent not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, bundle)
}

// importBundle handles POST /api/agents/import
// Provisions an agent bundle (agent + skills) in the current tenant.
func (h *BundleHandler) importBundle(w http.ResponseWriter, r *http.Request) {
	var bundle AgentBundle
	if err := json.NewDecoder(r.Body).Decode(&bundle); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid bundle payload")
		return
	}
	if bundle.FormatVersion == "" {
		respond.Error(w, http.StatusBadRequest, "missing formatVersion")
		return
	}
	if bundle.Agent.Name == "" {
		respond.Error(w, http.StatusBadRequest, "bundle.agent.name is required")
		return
	}
	result, err := h.importer.Import(r.Context(), bundle)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, result)
}
