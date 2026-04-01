// Package tenant implements the tenant provisioning domain.
package tenant

import "time"

// Status represents the provisioning status of a tenant.
type Status string

const (
	StatusActive             Status = "ACTIVE"
	StatusProvisioningFailed Status = "PROVISIONING_FAILED"
)

// Tenant is the domain entity for a platform tenant.
// The ID field doubles as the Keycloak realm name (kebab-case slug).
type Tenant struct {
	ID        string    // slug, e.g. "my-company"
	Name      string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}
