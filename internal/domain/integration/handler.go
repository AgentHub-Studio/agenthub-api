package integration

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

type integrationService interface {
	List(ctx context.Context, req pagination.PageRequest, filters ListFilters) (pagination.Page[Response], error)
}

// Handler exposes integration catalog endpoints.
type Handler struct {
	svc integrationService
}

// NewHandler creates a new integration handler.
func NewHandler(svc integrationService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts integration routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/integrations", h.list)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	filters, err := parseFilters(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.svc.List(r.Context(), pagination.ParsePageRequest(r), filters)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, page)
}

func parseFilters(r *http.Request) (ListFilters, error) {
	var filters ListFilters
	query := r.URL.Query()

	if rawType := query.Get("type"); rawType != "" {
		parsed := IntegrationType(rawType)
		switch parsed {
		case IntegrationTypeHTTPAPI, IntegrationTypeDatabaseQuery, IntegrationTypeMCP:
			filters.Type = &parsed
		default:
			return ListFilters{}, errInvalidFilter("type")
		}
	}

	if rawEnabled := query.Get("enabled"); rawEnabled != "" {
		parsed, err := strconv.ParseBool(rawEnabled)
		if err != nil {
			return ListFilters{}, errInvalidFilter("enabled")
		}
		filters.Enabled = &parsed
	}

	if rawOrigin := query.Get("origin"); rawOrigin != "" {
		parsed := IntegrationOrigin(rawOrigin)
		switch parsed {
		case IntegrationOriginLegacy, IntegrationOriginGenerated, IntegrationOriginManual:
			filters.Origin = &parsed
		default:
			return ListFilters{}, errInvalidFilter("origin")
		}
	}

	return filters, nil
}

func errInvalidFilter(name string) error {
	return &filterError{name: name}
}

type filterError struct{ name string }

func (e *filterError) Error() string { return "invalid " + e.name + " filter" }
