package keycloak

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"
)

// defaultRoles are the Keycloak realm roles created for every new tenant.
var defaultRoles = []string{"admin", "user", "mcp-client-runtime", "PROXY_SERVICE"}

const workloadAudienceMapperName = "agenthub-mcp-runtime-audience"

// Config holds configuration for the Keycloak Admin API provisioner.
type Config struct {
	BaseURL          string // e.g. http://keycloak.internal:8080
	AdminUsername    string
	AdminPassword    string
	AdminClientID    string // default: admin-cli
	AdminRealm       string // default: master
	FrontendClient   string // default: agenthub-frontend
	WorkloadClientID string // default: agenthub-api
	WorkloadAudience string // default: agenthub-mcp-client-runtime
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
	if cfg.WorkloadClientID == "" {
		cfg.WorkloadClientID = "agenthub-api"
	}
	if cfg.WorkloadAudience == "" {
		cfg.WorkloadAudience = "agenthub-mcp-client-runtime"
	}
	return &Provisioner{
		cfg: cfg,
		// 120s — under sustained load (BDD ladder spinning up tenants
		// back-to-back) Keycloak admin-cli can block >30s on a single
		// POST /admin/realms while it imports the AgentHub realm template.
		// A 30s ceiling caused PROVISIONING_FAILED rows that left tenants
		// realm-less and no automatic retry to recover them.
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// ProvisionRealm creates the tenant realm and returns its unique internal
// workload credential so that the caller can store it encrypted.
func (p *Provisioner) ProvisionRealm(ctx context.Context, tenantID string, _ string) (workloadidentity.Credential, error) {
	if err := p.createRealm(ctx, tenantID); err != nil {
		return workloadidentity.Credential{}, fmt.Errorf("keycloak provision: create realm: %w", err)
	}
	if err := p.createClient(ctx, tenantID); err != nil {
		return workloadidentity.Credential{}, fmt.Errorf("keycloak provision: create client: %w", err)
	}
	credential, err := p.EnsureWorkloadIdentity(ctx, tenantID)
	if err != nil {
		return workloadidentity.Credential{}, fmt.Errorf("keycloak provision: workload identity: %w", err)
	}
	if err := p.createRealmRoles(ctx, tenantID, defaultRoles); err != nil {
		return workloadidentity.Credential{}, fmt.Errorf("keycloak provision: create roles: %w", err)
	}
	return credential, nil
}

// EnsureWorkloadIdentity creates or repairs the tenant-local Keycloak service
// account used by agenthub-api to call the MCP runtime.
func (p *Provisioner) EnsureWorkloadIdentity(ctx context.Context, tenantID string) (workloadidentity.Credential, error) {
	secret, err := newWorkloadClientSecret()
	if err != nil {
		return workloadidentity.Credential{}, err
	}
	payload := map[string]any{
		"clientId":                  p.cfg.WorkloadClientID,
		"name":                      p.cfg.WorkloadClientID,
		"enabled":                   true,
		"publicClient":              false,
		"clientAuthenticatorType":   "client-secret",
		"secret":                    secret,
		"protocol":                  "openid-connect",
		"serviceAccountsEnabled":    true,
		"standardFlowEnabled":       false,
		"directAccessGrantsEnabled": false,
		"implicitFlowEnabled":       false,
		"fullScopeAllowed":          false,
		"protocolMappers":           []map[string]any{p.audienceMapperPayload()},
	}
	resp, err := p.adminRequest(ctx, http.MethodPost, fmt.Sprintf("/admin/realms/%s/clients", url.PathEscape(tenantID)), payload)
	if err != nil {
		return workloadidentity.Credential{}, err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return workloadidentity.Credential{}, fmt.Errorf("create workload client %d: %s", resp.StatusCode, body)
	}
	created := resp.StatusCode == http.StatusCreated
	if err := resp.Body.Close(); err != nil {
		return workloadidentity.Credential{}, fmt.Errorf("close workload client response: %w", err)
	}

	internalID, err := p.findClientInternalID(ctx, tenantID, p.cfg.WorkloadClientID)
	if err != nil {
		return workloadidentity.Credential{}, err
	}
	if err := p.ensureAudienceMapper(ctx, tenantID, internalID); err != nil {
		return workloadidentity.Credential{}, err
	}
	if !created {
		secret, err = p.getClientSecret(ctx, tenantID, internalID)
		if err != nil {
			return workloadidentity.Credential{}, err
		}
	}
	return workloadidentity.Credential{ClientID: p.cfg.WorkloadClientID, ClientSecret: secret}, nil
}

func newWorkloadClientSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate workload client secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func (p *Provisioner) findClientInternalID(ctx context.Context, tenantID, clientID string) (string, error) {
	resp, err := p.adminRequest(ctx, http.MethodGet, fmt.Sprintf("/admin/realms/%s/clients?clientId=%s", url.PathEscape(tenantID), url.QueryEscape(clientID)), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("find workload client %d: %s", resp.StatusCode, body)
	}
	var clients []struct {
		ID       string `json:"id"`
		ClientID string `json:"clientId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&clients); err != nil {
		return "", fmt.Errorf("decode workload clients: %w", err)
	}
	for _, client := range clients {
		if client.ClientID == clientID && client.ID != "" {
			return client.ID, nil
		}
	}
	return "", fmt.Errorf("workload client %q was not found after provisioning", clientID)
}

func (p *Provisioner) getClientSecret(ctx context.Context, tenantID, internalID string) (string, error) {
	resp, err := p.adminRequest(ctx, http.MethodGet, fmt.Sprintf("/admin/realms/%s/clients/%s/client-secret", url.PathEscape(tenantID), url.PathEscape(internalID)), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("read workload client secret %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode workload client secret: %w", err)
	}
	if result.Value == "" {
		return "", errors.New("workload client secret is empty")
	}
	return result.Value, nil
}

func (p *Provisioner) ensureAudienceMapper(ctx context.Context, tenantID, internalID string) error {
	path := fmt.Sprintf("/admin/realms/%s/clients/%s/protocol-mappers/models", url.PathEscape(tenantID), url.PathEscape(internalID))
	resp, err := p.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("list workload protocol mappers %d: %s", resp.StatusCode, body)
	}
	var mappers []struct {
		Name           string            `json:"name"`
		ProtocolMapper string            `json:"protocolMapper"`
		Config         map[string]string `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mappers); err != nil {
		return fmt.Errorf("decode workload protocol mappers: %w", err)
	}
	for _, mapper := range mappers {
		if mapper.Name != workloadAudienceMapperName {
			continue
		}
		if mapper.ProtocolMapper != "oidc-audience-mapper" || mapper.Config["included.custom.audience"] != p.cfg.WorkloadAudience || mapper.Config["access.token.claim"] != "true" {
			return errors.New("workload audience mapper exists with an incompatible configuration")
		}
		return nil
	}
	resp, err = p.adminRequest(ctx, http.MethodPost, path, p.audienceMapperPayload())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create workload audience mapper %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *Provisioner) audienceMapperPayload() map[string]any {
	return map[string]any{
		"name": workloadAudienceMapperName, "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper", "consentRequired": false,
		"config": map[string]string{"included.custom.audience": p.cfg.WorkloadAudience, "access.token.claim": "true", "id.token.claim": "false", "introspection.token.claim": "true"},
	}
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

// DeleteRealm removes a Keycloak realm. Tolerates 404 (already gone) for idempotency.
func (p *Provisioner) DeleteRealm(ctx context.Context, tenantID string) error {
	resp, err := p.adminRequest(ctx, http.MethodDelete,
		fmt.Sprintf("/admin/realms/%s", tenantID), nil)
	if err != nil {
		return fmt.Errorf("keycloak delete realm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete realm %d: %s", resp.StatusCode, body)
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
