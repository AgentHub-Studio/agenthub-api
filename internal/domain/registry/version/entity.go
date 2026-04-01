// Package version defines the package version domain entities and DTOs.
package version

import (
	"time"

	"github.com/google/uuid"
)

// PackageVersion is the domain entity for a package release.
type PackageVersion struct {
	ID            uuid.UUID
	PackageID     uuid.UUID
	Version       string
	Changelog     string
	StoragePath   string
	Checksum      string
	DownloadCount int
	PublishedAt   time.Time
	PublishedBy   string
}

// VersionResponse is the DTO returned to clients.
type VersionResponse struct {
	ID            uuid.UUID `json:"id"`
	PackageID     uuid.UUID `json:"packageId"`
	Version       string    `json:"version"`
	Changelog     string    `json:"changelog"`
	StoragePath   string    `json:"storagePath"`
	Checksum      string    `json:"checksum"`
	DownloadCount int       `json:"downloadCount"`
	PublishedAt   time.Time `json:"publishedAt"`
	PublishedBy   string    `json:"publishedBy"`
}

// ResponseFrom converts a PackageVersion entity to VersionResponse.
func ResponseFrom(v PackageVersion) VersionResponse {
	return VersionResponse(v)
}

// PublishVersionRequest is the DTO for publishing a new version.
type PublishVersionRequest struct {
	Version     string `json:"version"`
	Changelog   string `json:"changelog"`
	StoragePath string `json:"storagePath"`
	Checksum    string `json:"checksum"`
}
