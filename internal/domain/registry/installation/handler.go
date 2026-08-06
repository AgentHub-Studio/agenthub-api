package installation

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/apierror"
	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

const (
	maxUploadSize      = 100 << 20 // 100 MB
	maxUploadBodyBytes = maxUploadSize + (1 << 20)
	maxUploadFieldSize = 16 << 10
)

// installationService defines the methods used by Handler.
type installationService interface {
	ListAssets(ctx context.Context, packageID uuid.UUID, versionID *uuid.UUID) ([]AssetResponse, error)
	UploadAsset(ctx context.Context, packageID uuid.UUID, versionID *uuid.UUID, filename string, contentType string, reader io.Reader, size int64) (AssetResponse, error)
	DownloadURL(ctx context.Context, packageID, assetID uuid.UUID) (AssetDownloadResponse, error)
}

// packageReader verifies parent visibility before serving package assets.
type packageReader interface {
	GetAccessibleByID(ctx context.Context, id uuid.UUID, tenantID string) (pkg.PackageResponse, error)
}

// Handler exposes the HTTP interface for package assets.
type Handler struct {
	svc installationService
	pkg packageReader
}

// NewHandler creates a new installation Handler.
func NewHandler(svc installationService) *Handler {
	return &Handler{svc: svc}
}

// WithPackageReader wires the parent-package visibility checker.
func (h *Handler) WithPackageReader(p packageReader) *Handler {
	h.pkg = p
	return h
}

// RegisterReadRoutes mounts asset routes behind optional authentication. Public
// packages remain readable anonymously while PRIVATE packages require owner access.
func (h *Handler) RegisterReadRoutes(r chi.Router) {
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

	if _, ok := h.readablePackage(w, r, packageID); !ok {
		return
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
	tenantID := tenant.FromContext(r.Context())
	if tenantID == "" {
		apierror.Write(w, http.StatusUnauthorized, "missing tenant context")
		return
	}
	p, ok := h.readablePackage(w, r, packageID)
	if !ok {
		return
	}
	if p.AuthorTenantID != tenantID {
		apierror.Write(w, http.StatusForbidden, "not the package owner")
		return
	}

	file, fields, err := httputil.ReadLimitedMultipartFile(w, r, httputil.LimitedMultipartOptions{
		FileFields:    []string{"file"},
		MaxFileBytes:  maxUploadSize,
		MaxBodyBytes:  maxUploadBodyBytes,
		MaxFieldBytes: maxUploadFieldSize,
	})
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var versionID *uuid.UUID
	if raw := fields["versionId"]; raw != "" {
		vid, err := uuid.Parse(raw)
		if err != nil {
			apierror.Write(w, http.StatusBadRequest, "invalid versionId")
			return
		}
		versionID = &vid
	}

	asset, err := h.svc.UploadAsset(r.Context(), packageID, versionID, file.Filename, contentType, file.Reader(), file.Size)
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
	packageID, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid package id")
		return
	}
	if _, ok := h.readablePackage(w, r, packageID); !ok {
		return
	}
	assetID, err := uuid.Parse(chi.URLParam(r, "assetId"))
	if err != nil {
		apierror.Write(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	resp, err := h.svc.DownloadURL(r.Context(), packageID, assetID)
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

func (h *Handler) readablePackage(w http.ResponseWriter, r *http.Request, packageID uuid.UUID) (pkg.PackageResponse, bool) {
	if h.pkg == nil {
		return pkg.PackageResponse{AuthorTenantID: tenant.FromContext(r.Context())}, true
	}
	p, err := h.pkg.GetAccessibleByID(r.Context(), packageID, tenant.FromContext(r.Context()))
	if err != nil {
		apierror.Write(w, http.StatusNotFound, "package not found")
		return pkg.PackageResponse{}, false
	}
	return p, true
}
