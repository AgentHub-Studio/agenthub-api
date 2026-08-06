package a2a

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/grants", h.createGrant)
	r.Post("/invoke", h.invoke)
	return r
}

func (h *Handler) createGrant(w http.ResponseWriter, r *http.Request) {
	var req CreateGrantRequest
	if err := decodeRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.CreateGrant(r.Context(), tenantctx.FromContext(r.Context()), req)
	if err != nil {
		writeError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) invoke(w http.ResponseWriter, r *http.Request) {
	var req InvokeRequest
	if err := decodeRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		respond.Error(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	resp, err := h.svc.Invoke(r.Context(), tenantctx.FromContext(r.Context()), req)
	if err != nil {
		writeError(w, err)
		return
	}
	defer resp.Cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := writeSSEFrame(w, "a2a_started", resp.StartedEventData()); err != nil {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-resp.Events:
			if !ok {
				return
			}
			if err := writeSSEFrame(w, event.Type, event.Data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// decodeRequest accepts exactly one JSON value so a valid prefix cannot hide a
// second payload from the service boundary.
func decodeRequest(r *http.Request, target any) error {
	return httputil.DecodeSingleJSON(r.Body, target)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		respond.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrForbidden):
		respond.Error(w, http.StatusForbidden, "a2a grant is required")
	case errors.Is(err, ErrRateLimited):
		respond.Error(w, http.StatusTooManyRequests, "a2a rate limit exceeded")
	case errors.Is(err, ErrNotFound):
		respond.Error(w, http.StatusNotFound, "target agent not found")
	default:
		respond.Error(w, http.StatusInternalServerError, "a2a request failed")
	}
}

func writeSSEFrame(w io.Writer, eventType string, data json.RawMessage) error {
	eventType = safeSSELineField(eventType, "message")
	payload := safeSSEPayload(data)
	if _, err := fmt.Fprintf(w, "event: %s\n", eventType); err != nil {
		return err
	}
	for _, line := range strings.Split(payload, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func safeSSEPayload(data json.RawMessage) string {
	if len(data) == 0 {
		return "null"
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err == nil {
		return compact.String()
	}
	encoded, err := json.Marshal(string(data))
	if err != nil {
		return "null"
	}
	return string(encoded)
}

func safeSSELineField(value, fallback string) string {
	if value == "" || strings.IndexFunc(value, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0 {
		return fallback
	}
	return value
}
