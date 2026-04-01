package installation

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StorageBackend defines the interface for uploading and generating presigned URLs.
// This is a local abstraction — no agenthub-go-commons dependency.
type StorageBackend interface {
	// Upload stores an object and returns the storage path.
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (storagePath string, err error)
	// PresignedURL returns a time-limited URL for downloading an object.
	PresignedURL(ctx context.Context, storagePath string, expires time.Duration) (string, error)
}

// Service implements business logic for package assets.
type Service struct {
	repo    *Repository
	storage StorageBackend
}

// NewService creates a new installation Service.
func NewService(repo *Repository, storage StorageBackend) *Service {
	return &Service{repo: repo, storage: storage}
}

// ListAssets returns all assets for a package, optionally filtered by ?versionId= query param.
func (s *Service) ListAssets(ctx context.Context, packageID uuid.UUID, versionID *uuid.UUID) ([]AssetResponse, error) {
	assets, err := s.repo.ListByPackage(ctx, packageID, versionID)
	if err != nil {
		return nil, fmt.Errorf("service: list assets: %w", err)
	}
	responses := make([]AssetResponse, 0, len(assets))
	for _, a := range assets {
		responses = append(responses, ResponseFrom(a))
	}
	return responses, nil
}

// UploadAsset stores an uploaded file in the storage backend and persists the metadata.
func (s *Service) UploadAsset(
	ctx context.Context,
	packageID uuid.UUID,
	versionID *uuid.UUID,
	filename string,
	contentType string,
	reader io.Reader,
	size int64,
) (AssetResponse, error) {
	if strings.TrimSpace(filename) == "" {
		return AssetResponse{}, &ValidationError{Field: "filename", Message: "filename is required"}
	}

	// Build a unique storage key: packages/{packageId}/assets/{uuid}/{filename}
	storageKey := path.Join("packages", packageID.String(), "assets", uuid.New().String(), filename)

	storagePath, err := s.storage.Upload(ctx, storageKey, reader, size, contentType)
	if err != nil {
		return AssetResponse{}, fmt.Errorf("service: upload asset: %w", err)
	}

	a := PackageAsset{
		PackageID:   packageID,
		VersionID:   versionID,
		Filename:    filename,
		ContentType: contentType,
		StoragePath: storagePath,
		SizeBytes:   size,
	}

	created, err := s.repo.Create(ctx, a)
	if err != nil {
		return AssetResponse{}, fmt.Errorf("service: create asset record: %w", err)
	}
	return ResponseFrom(created), nil
}

// DownloadURL returns a presigned URL for downloading the asset.
func (s *Service) DownloadURL(ctx context.Context, assetID uuid.UUID) (AssetDownloadResponse, error) {
	asset, err := s.repo.GetByID(ctx, assetID)
	if err != nil {
		return AssetDownloadResponse{}, err
	}

	url, err := s.storage.PresignedURL(ctx, asset.StoragePath, 15*time.Minute)
	if err != nil {
		return AssetDownloadResponse{}, fmt.Errorf("service: generate presigned URL: %w", err)
	}
	return AssetDownloadResponse{URL: url}, nil
}

// ValidationError represents an input validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s — %s", e.Field, e.Message)
}
