// Package installation defines the package asset and installation domain entities and DTOs.
package installation

import (
	"time"

	"github.com/google/uuid"
)

// PackageAsset represents a binary or config file attached to a package/version.
type PackageAsset struct {
	ID          uuid.UUID
	PackageID   uuid.UUID
	VersionID   *uuid.UUID
	Filename    string
	ContentType string
	StoragePath string
	SizeBytes   int64
	Checksum    string
	CreatedAt   time.Time
}

// AssetResponse is the DTO returned to clients for a package asset.
type AssetResponse struct {
	ID          uuid.UUID  `json:"id"`
	PackageID   uuid.UUID  `json:"packageId"`
	VersionID   *uuid.UUID `json:"versionId,omitempty"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"contentType"`
	StoragePath string     `json:"storagePath"`
	SizeBytes   int64      `json:"sizeBytes"`
	Checksum    string     `json:"checksum"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// AssetDownloadResponse is the DTO returned when a client requests a download URL.
type AssetDownloadResponse struct {
	URL string `json:"url"`
}

// ResponseFrom converts a PackageAsset entity to AssetResponse.
func ResponseFrom(a PackageAsset) AssetResponse {
	return AssetResponse{
		ID:          a.ID,
		PackageID:   a.PackageID,
		VersionID:   a.VersionID,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		StoragePath: a.StoragePath,
		SizeBytes:   a.SizeBytes,
		Checksum:    a.Checksum,
		CreatedAt:   a.CreatedAt,
	}
}
