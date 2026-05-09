package skilleval

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the skill evaluation API.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts all skill-eval endpoints under /api/skill-evals.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/skill-evals", func(r chi.Router) {
		// Suite CRUD
		r.Get("/suites", h.listSuites)
		r.Post("/suites", h.createSuite)
		r.Get("/suites/{suiteId}", h.getSuite)
		r.Delete("/suites/{suiteId}", h.deleteSuite)

		// Cases within a suite
		r.Post("/suites/{suiteId}/cases", h.addCase)
		r.Delete("/cases/{caseId}", h.deleteCase)

		// Run a suite; list/get runs
		r.Post("/suites/{suiteId}/run", h.runSuite)
		r.Get("/suites/{suiteId}/runs", h.listRuns)
		r.Get("/runs/{runId}", h.getRun)
	})
}

func (h *Handler) listSuites(w http.ResponseWriter, r *http.Request) {
	var skillID *uuid.UUID
	if raw := r.URL.Query().Get("skillId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeEvalError(w, http.StatusBadRequest, "invalid skillId")
			return
		}
		skillID = &id
	}
	suites, err := h.svc.ListSuites(r.Context(), skillID)
	if err != nil {
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if suites == nil {
		suites = []SuiteResponse{}
	}
	writeEvalJSON(w, http.StatusOK, suites)
}

func (h *Handler) createSuite(w http.ResponseWriter, r *http.Request) {
	var req CreateSuiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.CreateSuite(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeEvalError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateName) {
			writeEvalError(w, http.StatusConflict, err.Error())
			return
		}
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeEvalJSON(w, http.StatusCreated, resp)
}

func (h *Handler) getSuite(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "suiteId"))
	if err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid suiteId")
		return
	}
	suite, cases, err := h.svc.GetSuite(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSuiteNotFound) {
			writeEvalError(w, http.StatusNotFound, "suite not found")
			return
		}
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeEvalJSON(w, http.StatusOK, map[string]any{
		"suite": suite,
		"cases": cases,
	})
}

func (h *Handler) deleteSuite(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "suiteId"))
	if err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid suiteId")
		return
	}
	if err := h.svc.DeleteSuite(r.Context(), id); err != nil {
		if errors.Is(err, ErrSuiteNotFound) {
			writeEvalError(w, http.StatusNotFound, "suite not found")
			return
		}
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) addCase(w http.ResponseWriter, r *http.Request) {
	suiteID, err := uuid.Parse(chi.URLParam(r, "suiteId"))
	if err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid suiteId")
		return
	}
	var req CreateCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.svc.AddCase(r.Context(), suiteID, req)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeEvalError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrSuiteNotFound) {
			writeEvalError(w, http.StatusNotFound, "suite not found")
			return
		}
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeEvalJSON(w, http.StatusCreated, resp)
}

func (h *Handler) deleteCase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "caseId"))
	if err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid caseId")
		return
	}
	if err := h.svc.DeleteCase(r.Context(), id); err != nil {
		if errors.Is(err, ErrCaseNotFound) {
			writeEvalError(w, http.StatusNotFound, "case not found")
			return
		}
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) runSuite(w http.ResponseWriter, r *http.Request) {
	suiteID, err := uuid.Parse(chi.URLParam(r, "suiteId"))
	if err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid suiteId")
		return
	}
	resp, err := h.svc.RunSuite(r.Context(), suiteID)
	if err != nil {
		if errors.Is(err, ErrSuiteNotFound) {
			writeEvalError(w, http.StatusNotFound, "suite not found")
			return
		}
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeEvalJSON(w, http.StatusOK, resp)
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	suiteID, err := uuid.Parse(chi.URLParam(r, "suiteId"))
	if err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid suiteId")
		return
	}
	runs, err := h.svc.ListRuns(r.Context(), suiteID)
	if err != nil {
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if runs == nil {
		runs = []RunResponse{}
	}
	writeEvalJSON(w, http.StatusOK, runs)
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runId"))
	if err != nil {
		writeEvalError(w, http.StatusBadRequest, "invalid runId")
		return
	}
	resp, err := h.svc.GetRun(r.Context(), runID)
	if err != nil {
		if errors.Is(err, ErrRunNotFound) {
			writeEvalError(w, http.StatusNotFound, "run not found")
			return
		}
		writeEvalError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeEvalJSON(w, http.StatusOK, resp)
}

func writeEvalJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeEvalError(w http.ResponseWriter, status int, msg string) {
	writeEvalJSON(w, status, map[string]string{"error": msg})
}
