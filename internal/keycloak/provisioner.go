package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// defaultRoles are the Keycloak realm roles created for every new tenant.
var defaultRoles = []string{"admin", "user", "mcp-client-runtime", "PROXY_SERVICE"}

// Config holds configuration for the Keycloak Admin API provisioner.
type Config struct {
	BaseURL        string // e.g. http://keycloak.internal:8080
	AdminUsername  string
	AdminPassword  string
	AdminClientID  string // default: admin-cli
	AdminRealm     string // default: master
	FrontendClient string // default: agenthub-frontend
}

// Provisioner implements tenant.ProvisioningClient using the Keycloak Admin REST API.
// On Create Tenant it: creates the realm, creates the agenthub-frontend client,
// and creates the default realm roles.
type Provisioner struct {
	cfg        Config
	httpClient *http.Client
}

// NewProvisioner creates a new Provisioner.
func NewProvisioner(cfg Config) *Provisioner {
	if cfg.AdminClientID == "" {
		cfg.AdminClientID = "admin-cli"
	}
	if cfg.AdminRealm == "" {
		cfg.AdminRealm = "master"
	}
	if cfg.FrontendClient == "" {
		cfg.FrontendClient = "agenthub-frontend"
	}
	return &Provisioner{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ProvisionRealm creates a Keycloak realm for the tenant with the default client and roles.
// It tolerates 409 Conflict (already exists) for idempotency.
func (p *Provisioner) ProvisionRealm(ctx context.Context, tenantID string, _ string) error {
	if err := p.createRealm(ctx, tenantID); err != nil {
		return fmt.Errorf("keycloak provision: create realm: %w", err)
	}
	if err := p.createClient(ctx, tenantID); err != nil {
		return fmt.Errorf("keycloak provision: create client: %w", err)
	}
	if err := p.createRealmRoles(ctx, tenantID, defaultRoles); err != nil {
		return fmt.Errorf("keycloak provision: create roles: %w", err)
	}
	return nil
}

func (p *Provisioner) getAdminToken(ctx context.Context) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		p.cfg.BaseURL, p.cfg.AdminRealm)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", p.cfg.AdminClientID)
	form.Set("username", p.cfg.AdminUsername)
	form.Set("password", p.cfg.AdminPassword)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token response %d: %s", resp.StatusCode, body)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	return result.AccessToken, nil
}

func (p *Provisioner) adminRequest(ctx context.Context, method, path string, bodyV any) (*http.Response, error) {
	token, err := p.getAdminToken(ctx)
	if err != nil {
		return nil, err
	}
	var bodyReader io.Reader
	if bodyV != nil {
		b, err := json.Marshal(bodyV)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.cfg.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if bodyV != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return p.httpClient.Do(req)
}

func (p *Provisioner) createRealm(ctx context.Context, tenantID string) error {
	payload := map[string]any{
		"realm":                  tenantID,
		"enabled":                true,
		"displayName":            tenantID,
		"loginTheme":             "agenthub-theme",
		"ssoSessionIdleTimeout":  1800,
		"accessTokenLifespan":    300,
		"registrationAllowed":    false,
		"loginWithEmailAllowed":  true,
		"duplicateEmailsAllowed": false,
		"resetPasswordAllowed":   true,
		"editUsernameAllowed":    false,
		"bruteForceProtected":    true,
	}
	resp, err := p.adminRequest(ctx, http.MethodPost, "/admin/realms", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		// Realm already exists — idempotent.
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create realm %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *Provisioner) createClient(ctx context.Context, tenantID string) error {
	payload := map[string]any{
		"clientId":                  p.cfg.FrontendClient,
		"name":                      p.cfg.FrontendClient,
		"enabled":                   true,
		"publicClient":              true,
		"standardFlowEnabled":       true,
		"directAccessGrantsEnabled": true,
		"redirectUris":              []string{"*"},
		"webOrigins":                []string{"*"},
	}
	resp, err := p.adminRequest(ctx, http.MethodPost,
		fmt.Sprintf("/admin/realms/%s/clients", tenantID), payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create client %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *Provisioner) createRealmRoles(ctx context.Context, tenantID string, roles []string) error {
	for _, role := range roles {
		payload := map[string]any{"name": role}
		resp, err := p.adminRequest(ctx, http.MethodPost,
			fmt.Sprintf("/admin/realms/%s/roles", tenantID), payload)
		if err != nil {
			return fmt.Errorf("role %q: %w", role, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusConflict {
			continue // already exists
		}
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create role %q %d: %s", role, resp.StatusCode, body)
		}
	}
	return nil
}
