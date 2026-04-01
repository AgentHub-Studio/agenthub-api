package dependency

import (
	"time"

	"github.com/google/uuid"
)

// PackageDependency represents a directed dependency edge between two packages.
type PackageDependency struct {
	ID                uuid.UUID
	PackageID         uuid.UUID
	DependencyID      uuid.UUID
	VersionConstraint string
	CreatedAt         time.Time
}

// DependencyResponse is the DTO returned to clients for a dependency edge.
type DependencyResponse struct {
	ID                uuid.UUID `json:"id"`
	PackageID         uuid.UUID `json:"packageId"`
	DependencyID      uuid.UUID `json:"dependencyId"`
	VersionConstraint string    `json:"versionConstraint"`
	CreatedAt         time.Time `json:"createdAt"`
}

// ResponseFrom converts a PackageDependency entity to a DependencyResponse DTO.
func ResponseFrom(d PackageDependency) DependencyResponse {
	return DependencyResponse(d)
}

// AddDependencyRequest is the payload for adding a dependency.
type AddDependencyRequest struct {
	DependencyID      uuid.UUID `json:"dependencyId"`
	VersionConstraint string    `json:"versionConstraint"`
}

// ResolvedDependency is a recursive tree node representing a package and its transitive deps.
type ResolvedDependency struct {
	PackageID         uuid.UUID            `json:"packageId"`
	Name              string               `json:"name"`
	Slug              string               `json:"slug"`
	VersionConstraint string               `json:"versionConstraint,omitempty"`
	Dependencies      []ResolvedDependency `json:"dependencies,omitempty"`
}
