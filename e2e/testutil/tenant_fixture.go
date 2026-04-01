//go:build e2e

package testutil

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TenantFixture provisions a test tenant and provides a ready-to-use JWT.
// Call Destroy() (typically via t.Cleanup) to remove the Keycloak realm.
type TenantFixture struct {
	Slug     string
	Name     string
	JWT      string
	keycloak *KeycloakClient
	t        *testing.T
}

// NewTenantFixture creates a tenant via the backend, waits for Keycloak realm
// provisioning, creates an admin user and obtains a JWT.
func NewTenantFixture(t *testing.T, backendURL, keycloakURL, adminUser, adminPass, userPass string) *TenantFixture {
	t.Helper()

	shortID := fmt.Sprintf("%d", time.Now().UnixNano())[:10]
	slug := "e2e-" + shortID
	name := "E2E Tenant " + shortID

	kc := NewKeycloakClient(keycloakURL, adminUser, adminPass)
	f := &TenantFixture{Slug: slug, Name: name, keycloak: kc, t: t}

	// 1. Create tenant
	noAuthClient := &http.Client{Timeout: 15 * time.Second}
	body := fmt.Sprintf(`{"name":%q,"slug":%q}`, name, slug)
	resp, err := noAuthClient.Post(backendURL+"/public/tenants", "application/json",
		strings.NewReader(body))
	if err != nil || resp.StatusCode != http.StatusCreated {
		sc := 0
		if resp != nil {
			sc = resp.StatusCode
			resp.Body.Close()
		}
		t.Fatalf("TenantFixture: create tenant %q: status %d err %v", slug, sc, err)
	}
	resp.Body.Close()

	// 2. Wait for Keycloak realm
	if err := kc.WaitForRealm(slug, 20*time.Second); err != nil {
		t.Fatalf("TenantFixture: %v", err)
	}

	// 3. Create admin user
	const adminUsername = "e2e-admin"
	if _, err := kc.CreateAdminUser(slug, adminUsername, userPass); err != nil {
		t.Fatalf("TenantFixture: create admin user: %v", err)
	}

	// 4. Obtain JWT
	jwt, err := kc.UserToken(slug, adminUsername, userPass)
	if err != nil {
		t.Fatalf("TenantFixture: get JWT: %v", err)
	}
	f.JWT = jwt

	t.Cleanup(f.Destroy)
	return f
}

// BearerToken returns the Authorization header value.
func (f *TenantFixture) BearerToken() string { return "Bearer " + f.JWT }

// Client returns an APIClient authenticated with this tenant's JWT.
func (f *TenantFixture) Client(t *testing.T, backendURL string) *APIClient {
	return NewAPIClient(t, backendURL, f.JWT)
}

// Destroy removes the Keycloak realm (idempotent).
func (f *TenantFixture) Destroy() {
	if err := f.keycloak.DeleteRealm(f.Slug); err != nil {
		f.t.Logf("TenantFixture: cleanup realm %q: %v", f.Slug, err)
	}
}

