package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// BundleHandler exposes bundle export/import HTTP endpoints.
type BundleHandler struct {
	exporter         *Exporter
	importer         *Importer
	portableSvc      Service
	portableSlugRepo PortableSkillSlugChecker
}

// NewBundleHandler creates a BundleHandler.
func NewBundleHandler(exporter *Exporter, importer *Importer) *BundleHandler {
	return &BundleHandler{exporter: exporter, importer: importer}
}

// PortableSkillSlugChecker validates documented Agent-as-Code skill slugs.
type PortableSkillSlugChecker interface {
	SlugExists(ctx context.Context, slug string) (bool, error)
}

// WithPortableYAML enables the documented Agent-as-Code YAML representation on
// the same routes as the legacy JSON bundle, selected by Accept/Content-Type.
func (h *BundleHandler) WithPortableYAML(svc Service, slugRepo PortableSkillSlugChecker) *BundleHandler {
	h.portableSvc = svc
	h.portableSlugRepo = slugRepo
	return h
}

// RegisterBundleRoutes mounts administrator-only bundle routes on the given router.
func (h *BundleHandler) RegisterBundleRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/agents/{id}/export", h.exportBundle)
		r.Post("/api/agents/import", h.importBundle)
	})
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
	if wantsPortableYAML(r) {
		portable := ToPortableFromBundle(bundle)
		data, err := MarshalPortableYAML(portable)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Content-Disposition", `attachment; filename="`+portable.Metadata.Slug+`.yaml"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}
	respond.JSON(w, http.StatusOK, bundle)
}

// importBundle handles POST /api/agents/import
// Provisions an agent bundle (agent + skills) in the current tenant.
func (h *BundleHandler) importBundle(w http.ResponseWriter, r *http.Request) {
	if isPortableYAMLRequest(r) {
		h.importPortableYAML(w, r)
		return
	}
	var bundle AgentBundle
	if err := httputil.DecodeSingleJSON(r.Body, &bundle); err != nil {
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

func (h *BundleHandler) importPortableYAML(w http.ResponseWriter, r *http.Request) {
	if h.portableSvc == nil {
		respond.Error(w, http.StatusUnsupportedMediaType, "portable yaml import is not configured")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "failed to read body")
		return
	}
	portable, err := UnmarshalPortableYAML(body)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if missing, err := h.missingSkillSlugs(r.Context(), portable.Spec.Skills); err != nil {
		slog.Error("agent: portable import skill validation failed", "err", err)
		respond.Error(w, http.StatusInternalServerError, "import failed")
		return
	} else if len(missing) > 0 {
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{"missing": missing})
		return
	}
	resp, err := h.portableSvc.Create(r.Context(), portable.ToCreateRequest())
	if errors.Is(err, ErrSlugConflict) {
		respond.Error(w, http.StatusConflict, "agent slug already exists")
		return
	}
	if errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrInvalidModelConfig) ||
		errors.Is(err, ErrInvalidSkillIDs) || errors.Is(err, ErrInvalidKnowledgeBaseIDs) ||
		errors.Is(err, ErrInvalidMCPServerIDs) {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		slog.Error("agent: portable import failed", "err", err)
		respond.Error(w, http.StatusInternalServerError, "import failed")
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *BundleHandler) missingSkillSlugs(ctx context.Context, slugs []string) ([]string, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	if h.portableSlugRepo == nil {
		return slugs, nil
	}
	var missing []string
	for _, slug := range slugs {
		exists, err := h.portableSlugRepo.SlugExists(ctx, slug)
		if err != nil {
			return nil, err
		}
		if !exists {
			missing = append(missing, slug)
		}
	}
	return missing, nil
}

func wantsPortableYAML(r *http.Request) bool {
	if strings.EqualFold(r.URL.Query().Get("format"), "yaml") {
		return true
	}
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "application/yaml") ||
		strings.Contains(accept, "application/x-yaml") ||
		strings.Contains(accept, "text/yaml")
}

func isPortableYAMLRequest(r *http.Request) bool {
	if strings.EqualFold(r.URL.Query().Get("format"), "yaml") {
		return true
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	return strings.Contains(contentType, "application/yaml") ||
		strings.Contains(contentType, "application/x-yaml") ||
		strings.Contains(contentType, "text/yaml")
}
