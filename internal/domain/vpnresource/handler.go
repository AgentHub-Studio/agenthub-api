package vpnresource

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// vpnService is the interface required by Handler.
type vpnService interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]VpnResource, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (VpnResource, error)
	Create(ctx context.Context, tenantID string, req CreateRequest) (VpnResource, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (VpnResource, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
	TestConnection(ctx context.Context, tenantID string, id uuid.UUID) (TestConnectionResponse, error)
	UploadOvpnConfig(ctx context.Context, tenantID string, id uuid.UUID, r io.Reader, size int64) (VpnResource, error)
	UploadAuthFile(ctx context.Context, tenantID string, id uuid.UUID, r io.Reader, size int64) (VpnResource, error)
}

// Handler handles HTTP requests for VPN resources.
type Handler struct {
	svc vpnService
}

// NewHandler creates a new Handler.
func NewHandler(svc vpnService) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the handler routes.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/test", h.testConnection)
	r.Post("/{id}/upload-config", h.uploadConfig)
	r.Post("/{id}/upload-auth", h.uploadAuth)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)

	items, total, err := h.svc.ListAll(r.Context(), tenantID, pr)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	dtos := make([]VpnResourceResponse, len(items))
	for i, v := range items {
		dtos[i] = ResponseFrom(v)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.svc.Create(r.Context(), tenantID, req)
	if err != nil {
		// ErrValidation veio do service quando o request omite name.
		// Sem essa branch, body {} criava VpnResource com name=""
		// (lixo persistido) e o usuário recebia 500 silencioso.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateName) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusCreated, ResponseFrom(created))
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	v, err := h.svc.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(v))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.svc.Update(r.Context(), tenantID, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		// Update agora valida name (consistente com Create); 422
		// quando o operador envia body parcial sem name.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(updated))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Delete(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.NoContent(w)
}

func (h *Handler) testConnection(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	res, err := h.svc.TestConnection(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	respond.JSON(w, http.StatusOK, res)
}

// uploadConfig accepts a multipart upload of a .ovpn configuration file.
func (h *Handler) uploadConfig(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	const maxSize = 1 << 20 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		respond.Error(w, http.StatusBadRequest, "request too large or not multipart")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "field 'file' is required")
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Debug("vpn: close ovpn upload", "err", closeErr)
		}
	}()

	updated, err := h.svc.UploadOvpnConfig(r.Context(), tenantID, id, file, header.Size)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		// Bug 258: validation (size cap, missing remote/dev directive) usa
		// ErrValidation; demais (read I/O ou MinIO upload errors) podem
		// conter o endpoint interno do MinIO — sanitizar.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("vpn: upload ovpn config failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "upload failed")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(updated))
}

// uploadAuth accepts a multipart upload of a VPN auth file (username/password).
func (h *Handler) uploadAuth(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	const maxSize = 4096
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		respond.Error(w, http.StatusBadRequest, "request too large or not multipart")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "field 'file' is required")
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Debug("vpn: close auth upload", "err", closeErr)
		}
	}()

	updated, err := h.svc.UploadAuthFile(r.Context(), tenantID, id, io.Reader(file), header.Size)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, err.Error())
			return
		}
		// Bug 258: igual UploadOvpnConfig — distinguir validation
		// de upload/IO errors do MinIO.
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("vpn: upload auth file failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "upload failed")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(updated))
}
