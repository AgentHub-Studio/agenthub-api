package keycloak

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// CreateRealm provisions a new realm for a tenant.
func (c *AdminClient) CreateRealm(ctx context.Context, tenantID string) error {
	body := map[string]any{
		"realm":                  tenantID,
		"enabled":                true,
		"registrationAllowed":    false,
		"loginWithEmailAllowed":  true,
		"duplicateEmailsAllowed": false,
		"sslRequired":            "external",
		"defaultLocale":          "pt-BR",
	}

	resp, err := c.doJSON(ctx, http.MethodPost, "/admin/realms", body)
	if err != nil {
		return fmt.Errorf("keycloak: create realm %q: %w", tenantID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil // already exists
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: create realm %q status %d: %s", tenantID, resp.StatusCode, b)
	}
	return nil
}

// CreateClient creates the agenthub-frontend public client in the realm.
func (c *AdminClient) CreateClient(ctx context.Context, tenantID string) error {
	body := map[string]any{
		"clientId":                  c.cfg.ClientID,
		"enabled":                   true,
		"publicClient":              true,
		"directAccessGrantsEnabled": true,
		"standardFlowEnabled":       true,
		"redirectUris":              []string{"*"},
		"webOrigins":                []string{"*"},
	}

	resp, err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/admin/realms/%s/clients", tenantID), body)
	if err != nil {
		return fmt.Errorf("keycloak: create client in realm %q: %w", tenantID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: create client status %d: %s", resp.StatusCode, b)
	}
	return nil
}

// CreateRealmRoles creates the standard roles in the realm.
func (c *AdminClient) CreateRealmRoles(ctx context.Context, tenantID string, roles []string) error {
	for _, role := range roles {
		body := map[string]any{"name": role}
		resp, err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/admin/realms/%s/roles", tenantID), body)
		if err != nil {
			return fmt.Errorf("keycloak: create role %q in realm %q: %w", role, tenantID, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("keycloak: create role %q status %d", role, resp.StatusCode)
		}
	}
	return nil
}

// ProvisionTenant creates realm + frontend client + standard roles.
// This is the one-shot operation called during tenant registration.
func (c *AdminClient) ProvisionTenant(ctx context.Context, tenantID string) error {
	if err := c.CreateRealm(ctx, tenantID); err != nil {
		return err
	}
	if err := c.CreateClient(ctx, tenantID); err != nil {
		return err
	}
	defaultRoles := []string{"admin", "user", "mcp-client-runtime", "PROXY_SERVICE"}
	return c.CreateRealmRoles(ctx, tenantID, defaultRoles)
}
