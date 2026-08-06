package embedding

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/embeddings", h.embed)
}

type embedRequest struct {
	Text     string            `json:"text"`
	Provider string            `json:"provider"`
	Body     *embedRequestBody `json:"body"`
}

type embedRequestBody struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
}

func (req embedRequest) normalized() (text, provider string) {
	text = req.Text
	provider = req.Provider
	if req.Body != nil {
		if text == "" {
			text = req.Body.Text
		}
		if provider == "" {
			provider = req.Body.Provider
		}
	}
	return text, provider
}

func (h *Handler) embed(w http.ResponseWriter, r *http.Request) {
	var req embedRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	text, provider := req.normalized()
	result, err := h.service.Embed(r.Context(), text, provider)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyText):
			respond.Error(w, http.StatusUnprocessableEntity, "text is required")
		case errors.Is(err, ErrUnknownProvider):
			respond.Error(w, http.StatusUnprocessableEntity, "unknown embedding provider")
		default:
			respond.Error(w, http.StatusBadGateway, "embedding provider failed")
		}
		return
	}

	respond.JSON(w, http.StatusOK, result)
}
