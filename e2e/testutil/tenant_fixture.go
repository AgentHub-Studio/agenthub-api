//go:build e2e

package testutil

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var tenantFixtureSequence atomic.Uint64

func nextTenantFixtureID(now time.Time) string {
	return fmt.Sprintf("%d-%d", now.UnixNano(), tenantFixtureSequence.Add(1))
}

func tenantFixtureCreateBody(name, slug string) string {
	return fmt.Sprintf(`{"name":%q,"id":%q}`, name, slug)
}

// TenantFixture provisions a test tenant and provides a ready-to-use JWT.
// Destroy removes the tenant through the core administrative API.
type TenantFixture struct {
	Slug         string
	Name         string
	JWT          string
	backendURL   string
	userPassword string
	keycloak     *KeycloakClient
	t            *testing.T
}

// NewTenantFixture creates a tenant via the backend, waits for Keycloak realm
// provisioning, creates an admin user and obtains a JWT.
func NewTenantFixture(t *testing.T, backendURL, keycloakURL, adminUser, adminPass, userPass string) *TenantFixture {
	t.Helper()

	fixtureID := nextTenantFixtureID(time.Now())
	slug := "e2e-" + fixtureID
	name := "E2E Tenant " + fixtureID

	kc := NewKeycloakClient(keycloakURL, adminUser, adminPass)
	f := &TenantFixture{
		Slug:         slug,
		Name:         name,
		backendURL:   backendURL,
		userPassword: userPass,
		keycloak:     kc,
		t:            t,
	}

	// 1. Create tenant
	// Tenant creation waits synchronously for Keycloak provisioning, whose service
	// contract permits six minutes. Keep a small margin for the HTTP response.
	noAuthClient := &http.Client{Timeout: 7 * time.Minute}
	body := tenantFixtureCreateBody(name, slug)
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
	t.Cleanup(f.Destroy)

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

	return f
}

// BearerToken returns the Authorization header value.
func (f *TenantFixture) BearerToken() string { return "Bearer " + f.JWT }

// Client returns an APIClient authenticated with this tenant's JWT.
func (f *TenantFixture) Client(t *testing.T, backendURL string) *APIClient {
	return NewAPIClient(t, backendURL, f.JWT)
}

// Destroy removes the tenant through the core administrative API. It falls
// back to the realm deletion only when the full cleanup flow is unavailable.
func (f *TenantFixture) Destroy() {
	if err := f.deleteTenantViaCoreAPI(); err == nil {
		return
	} else {
		if f.t != nil {
			f.t.Errorf("TenantFixture: cleanup tenant %q through core API: %v", f.Slug, err)
		}
	}
	if err := f.keycloak.DeleteRealm(f.Slug); err != nil {
		if f.t != nil {
			f.t.Logf("TenantFixture: fallback cleanup realm %q: %v", f.Slug, err)
		}
	}
}

func (f *TenantFixture) deleteTenantViaCoreAPI() error {
	const coreRealm = "core"
	cleanupUsername := "e2e-cleanup-" + f.Slug
	cleanupUserID, err := f.keycloak.CreateAdminUser(coreRealm, cleanupUsername, f.userPassword)
	if err != nil {
		return fmt.Errorf("create core cleanup user: %w", err)
	}
	defer func() {
		if err := f.keycloak.DeleteUser(coreRealm, cleanupUserID); err != nil && f.t != nil {
			f.t.Logf("TenantFixture: cleanup core user %q: %v", cleanupUsername, err)
		}
	}()

	token, err := f.keycloak.UserToken(coreRealm, cleanupUsername, f.userPassword)
	if err != nil {
		return fmt.Errorf("get core cleanup token: %w", err)
	}
	req, err := http.NewRequest(http.MethodDelete,
		f.backendURL+"/api/admin/tenants/"+url.PathEscape(f.Slug), nil)
	if err != nil {
		return fmt.Errorf("build tenant cleanup request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := f.keycloak.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete tenant through core API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete tenant through core API %d: %s", resp.StatusCode, raw)
	}
	return nil
}
