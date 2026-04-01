// Package pkg defines the package registry domain entities, DTOs and repository.
package pkg

import (
	"time"

	"github.com/google/uuid"
)

// PackageType represents the type of a registry package.
type PackageType string

const (
	PackageTypeAgent         PackageType = "AGENT"
	PackageTypeSkill         PackageType = "SKILL"
	PackageTypeTool          PackageType = "TOOL"
	PackageTypeKnowledgeBase PackageType = "KNOWLEDGE_BASE"
)

// PackageVisibility represents the visibility of a registry package.
type PackageVisibility string

const (
	PackageVisibilityPublic  PackageVisibility = "PUBLIC"
	PackageVisibilityPrivate PackageVisibility = "PRIVATE"
)

// Package is the domain entity for a registry package.
type Package struct {
	ID              uuid.UUID
	Name            string
	Slug            string
	Description     string
	Type            PackageType
	Visibility      PackageVisibility
	AuthorTenantID  string
	DownloadCount   int
	LatestVersion   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PackageResponse is the DTO returned to clients.
type PackageResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	Description     string    `json:"description"`
	Type            string    `json:"type"`
	Visibility      string    `json:"visibility"`
	AuthorTenantID  string    `json:"authorTenantId"`
	DownloadCount   int       `json:"downloadCount"`
	LatestVersion   string    `json:"latestVersion"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ResponseFrom converts a Package entity to PackageResponse.
func ResponseFrom(p Package) PackageResponse {
	return PackageResponse{
		ID:             p.ID,
		Name:           p.Name,
		Slug:           p.Slug,
		Description:    p.Description,
		Type:           string(p.Type),
		Visibility:     string(p.Visibility),
		AuthorTenantID: p.AuthorTenantID,
		DownloadCount:  p.DownloadCount,
		LatestVersion:  p.LatestVersion,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

// CreatePackageRequest is the DTO for creating a new package.
type CreatePackageRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Visibility  string `json:"visibility"`
}

// UpdatePackageRequest is the DTO for updating a package.
type UpdatePackageRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Visibility  *string `json:"visibility"`
}
