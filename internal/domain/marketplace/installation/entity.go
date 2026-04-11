// Package installation provides marketplace one-click install domain logic.
package installation

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an installation is not found.
var ErrNotFound = errors.New("installation not found")

// InstallStatus represents the state of an installation.
type InstallStatus string

const (
	InstallStatusInstalled   InstallStatus = "INSTALLED"
	InstallStatusUninstalled InstallStatus = "UNINSTALLED"
)

// Installation is the domain entity for a per-tenant package installation.
type Installation struct {
	ID             uuid.UUID
	TenantID       string
	PackageID      uuid.UUID
	PackageVersion string
	Status         InstallStatus
	Hydrated       bool
	InstalledAt    time.Time
}

// InstallResponse is the DTO returned to clients.
type InstallResponse struct {
	ID             uuid.UUID     `json:"id"`
	TenantID       string        `json:"tenantId"`
	PackageID      uuid.UUID     `json:"packageId"`
	PackageVersion string        `json:"packageVersion"`
	Status         InstallStatus `json:"status"`
	Hydrated       bool          `json:"hydrated"`
	InstalledAt    time.Time     `json:"installedAt"`
}

// ResponseFrom converts an Installation to InstallResponse.
func ResponseFrom(i Installation) InstallResponse {
	return InstallResponse(i)
}

// InstallRequest is the DTO for installing a package.
type InstallRequest struct {
	PackageID      uuid.UUID `json:"packageId"`
	PackageVersion string    `json:"packageVersion"`
}
