package tenant

import "time"

// TenantResponse is the JSON response envelope for a tenant.
type TenantResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ResponseFrom converts a Tenant entity to TenantResponse.
func ResponseFrom(t Tenant) TenantResponse {
	return TenantResponse{
		ID:        t.ID,
		Name:      t.Name,
		Status:    string(t.Status),
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// CreateTenantRequest is the JSON body for tenant creation.
type CreateTenantRequest struct {
	// ID is the kebab-case slug used as realm name in Keycloak.
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ExistsResponse is returned by the /exists endpoint.
type ExistsResponse struct {
	Exists bool `json:"exists"`
}
