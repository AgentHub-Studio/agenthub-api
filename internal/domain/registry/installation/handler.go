package installation

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/apierror"
)

const maxUploadSize = 100 << 20 // 100 MB

// installationService defines the methods used by Handler.
type installationService interface {
	ListAssets(ctx context.Context, packageID uuid.UUID, versionID *uuid.UUID) ([]AssetResponse, error)
	UploadAsset(ctx context.Context, packageID uuid.UUID, versionID *uuid.UUID, filename string, contentType string, reader io.Reader, size int64) (AssetResponse, error)
	DownloadURL(ctx context.Context, assetID uuid.UUID) (AssetDownloadResponse, error)
}

// packageExister verifies parent package existence (bug 210 batch).
type packageExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// Handler exposes the HTTP interface for package assets.
type Handler struct {
	svc installationService
	pkg packageExister
}

// NewHandler creates a new installation Handler.
func NewHandler(svc installationService) *Handler {
	return &Handler{svc: svc}
}

// WithPackageExister wires the parent-package existence checker.
func (h *Handler) WithPackageExister(p packageExister) *Handler {
	h.pkg = p
	return h
}

// RegisterPublicRoutes mounts read-only asset routes.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/api/packages/{packageId}/assets", h.listAssets)
	r.Get("/api/packages/{packageId}/assets/{assetId}/download", h.download)
}

// RegisterProtectedRoutes mounts asset write routes (require authentication).
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Post("/api/packages/{packageId}/assets", h.uploadAsset)
}

// listAssets godoc — GET /api/packages/{packageId}/assets?versionId=<uuid>
func (h *Handler) listAssets(w http.ResponseWriter, r *http.Request) {
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}

	if h.pkg != nil {
		if err := h.pkg.GetByID(r.Context(), packageID); err != nil {
			apierror.Write(w, http.StatusNotFound, "package not found")
			return
		}
	}

	var versionID *uuid.UUID
	if raw := r.URL.Query().Get("versionId"); raw != "" {
		vid, err := uuid.Parse(raw)
		if err != nil {
			apierror.Write(w, http.StatusBadRequest, "invalid versionId")
			return
		}
		versionID = &vid
	}

	assets, err := h.svc.ListAssets(r.Context(), packageID, versionID)
	if err != nil {
		apierror.Write(w, http.StatusInternalServerError, "failed to list assets")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, assets)
}

// uploadAsset godoc — POST /api/packages/{packageId}/assets  (multipart/form-data)
func (h *Handler) uploadAsset(w http.ResponseWriter, r *http.Request) {
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		apierror.Write(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer func() { _ = file.Close() }()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var versionID *uuid.UUID
	if raw := r.FormValue("versionId"); raw != "" {
		vid, err := uuid.Parse(raw)
		if err != nil {
			apierror.Write(w, http.StatusBadRequest, "invalid versionId")
			return
		}
		versionID = &vid
	}

	asset, err := h.svc.UploadAsset(r.Context(), packageID, versionID, header.Filename, contentType, file, header.Size)
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			apierror.Write(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to upload asset")
		return
	}
	apierror.WriteJSON(w, http.StatusCreated, asset)
}

// download godoc — GET /api/packages/{packageId}/assets/{assetId}/download
func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "assetId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	resp, err := h.svc.DownloadURL(r.Context(), assetID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierror.Write(w, http.StatusNotFound, "asset not found")
			return
		}
		apierror.Write(w, http.StatusInternalServerError, "failed to generate download URL")
		return
	}
	apierror.WriteJSON(w, http.StatusOK, resp)
}
