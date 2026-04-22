package tenantsignup

// SignupRequest is the body of POST /api/tenants/signup.
// Visible to unauthenticated visitors on the public signup page, so fields
// are kept minimal.
type SignupRequest struct {
	TenantID        string `json:"tenantId"`
	TenantName      string `json:"tenantName"`
	AdminEmail      string `json:"adminEmail"`
	AdminFirstName  string `json:"adminFirstName"`
	AdminLastName   string `json:"adminLastName,omitempty"`
}

// SignupResponse is the JSON returned to the frontend after a successful signup.
// It includes a one-time temporary password the user should change on first login.
type SignupResponse struct {
	TenantID     string `json:"tenantId"`
	Username     string `json:"username"`
	LoginURL     string `json:"loginUrl"`
	TempPassword string `json:"tempPassword"`
}
