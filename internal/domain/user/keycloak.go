package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ErrNotFound is returned when the user cannot be found.
var ErrNotFound = errors.New("user: not found")

// ErrAlreadyExists is returned when the username/email is already taken.
var ErrAlreadyExists = errors.New("user: already exists")

// KeycloakUserClient defines the Keycloak Admin API operations for users.
type KeycloakUserClient interface {
	ListUsers(ctx context.Context, tenantID string) ([]User, error)
	GetUser(ctx context.Context, tenantID string, userID string) (User, error)
	CreateUser(ctx context.Context, tenantID string, req CreateUserRequest) (User, error)
	UpdateUser(ctx context.Context, tenantID string, userID string, req UpdateUserRequest) (User, error)
	DeleteUser(ctx context.Context, tenantID string, userID string) error
	AssignRole(ctx context.Context, tenantID string, userID string, role string) error
	RemoveRole(ctx context.Context, tenantID string, userID string, role string) error
}

// keycloakClient is the HTTP implementation of KeycloakUserClient.
type keycloakClient struct {
	baseURL        string
	adminUsername  string
	adminPassword  string
	adminClientID  string
	adminRealm     string
	frontendClient string
	httpClient     *http.Client
}

// KeycloakClientConfig holds configuration for the Keycloak Admin API client.
type KeycloakClientConfig struct {
	BaseURL        string // e.g. http://keycloak.internal:8080
	AdminUsername  string
	AdminPassword  string
	AdminClientID  string // defaults to admin-cli
	AdminRealm     string // defaults to master
	FrontendClient string // defaults to agenthub-frontend
}

// NewKeycloakUserClient creates a new Keycloak Admin API user client.
func NewKeycloakUserClient(cfg KeycloakClientConfig) KeycloakUserClient {
	if cfg.AdminClientID == "" {
		cfg.AdminClientID = "admin-cli"
	}
	if cfg.AdminRealm == "" {
		cfg.AdminRealm = "master"
	}
	if cfg.FrontendClient == "" {
		cfg.FrontendClient = "agenthub-frontend"
	}
	return &keycloakClient{
		baseURL:        cfg.BaseURL,
		adminUsername:  cfg.AdminUsername,
		adminPassword:  cfg.AdminPassword,
		adminClientID:  cfg.AdminClientID,
		adminRealm:     cfg.AdminRealm,
		frontendClient: cfg.FrontendClient,
		httpClient:     &http.Client{},
	}
}

// getAdminToken obtains a short-lived admin access token from the master realm.
func (c *keycloakClient) getAdminToken(ctx context.Context) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, c.adminRealm)
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", c.adminClientID)
	data.Set("username", c.adminUsername)
	data.Set("password", c.adminPassword)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("keycloak: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak: token response %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("keycloak: parse token: %w", err)
	}
	return result.AccessToken, nil
}

func (c *keycloakClient) adminRequest(ctx context.Context, method, path string, bodyV any) (*http.Response, error) {
	token, err := c.getAdminToken(ctx)
	if err != nil {
		return nil, err
	}
	var bodyReader io.Reader
	if bodyV != nil {
		b, err := json.Marshal(bodyV)
		if err != nil {
			return nil, fmt.Errorf("keycloak: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("keycloak: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if bodyV != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

// kcUser is the Keycloak representation of a user (subset).
type kcUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Enabled   bool   `json:"enabled"`
}

func kcUserToUser(k kcUser, roles []string) User {
	return User{
		ID:        k.ID,
		Username:  k.Username,
		Email:     k.Email,
		FirstName: k.FirstName,
		LastName:  k.LastName,
		Enabled:   k.Enabled,
		Roles:     roles,
	}
}

func (c *keycloakClient) getClientUUID(ctx context.Context, tenantID string) (string, error) {
	path := fmt.Sprintf("/admin/realms/%s/clients?clientId=%s", tenantID, c.frontendClient)
	resp, err := c.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak: get clients %d: %s", resp.StatusCode, string(body))
	}
	var clients []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &clients); err != nil || len(clients) == 0 {
		return "", fmt.Errorf("keycloak: client %q not found in realm %s", c.frontendClient, tenantID)
	}
	return clients[0].ID, nil
}

func (c *keycloakClient) getUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	clientUUID, err := c.getClientUUID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/clients/%s", tenantID, userID, clientUUID)
	resp, err := c.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak: get user roles %d: %s", resp.StatusCode, string(body))
	}
	var roles []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, fmt.Errorf("keycloak: parse roles: %w", err)
	}
	names := make([]string, len(roles))
	for i, role := range roles {
		names[i] = role.Name
	}
	return names, nil
}

func (c *keycloakClient) ListUsers(ctx context.Context, tenantID string) ([]User, error) {
	path := fmt.Sprintf("/admin/realms/%s/users", tenantID)
	resp, err := c.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak: list users %d: %s", resp.StatusCode, string(body))
	}
	var kcUsers []kcUser
	if err := json.Unmarshal(body, &kcUsers); err != nil {
		return nil, fmt.Errorf("keycloak: parse users: %w", err)
	}
	users := make([]User, len(kcUsers))
	for i, ku := range kcUsers {
		roles, _ := c.getUserRoles(ctx, tenantID, ku.ID)
		users[i] = kcUserToUser(ku, roles)
	}
	return users, nil
}

func (c *keycloakClient) GetUser(ctx context.Context, tenantID string, userID string) (User, error) {
	path := fmt.Sprintf("/admin/realms/%s/users/%s", tenantID, userID)
	resp, err := c.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return User{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("keycloak: get user %d: %s", resp.StatusCode, string(body))
	}
	var ku kcUser
	if err := json.Unmarshal(body, &ku); err != nil {
		return User{}, fmt.Errorf("keycloak: parse user: %w", err)
	}
	roles, _ := c.getUserRoles(ctx, tenantID, ku.ID)
	return kcUserToUser(ku, roles), nil
}

func (c *keycloakClient) CreateUser(ctx context.Context, tenantID string, req CreateUserRequest) (User, error) {
	type kcCreateReq struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Enabled   bool   `json:"enabled"`
		Credentials []struct {
			Type      string `json:"type"`
			Value     string `json:"value"`
			Temporary bool   `json:"temporary"`
		} `json:"credentials"`
	}
	body := kcCreateReq{
		Username:  req.Username,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Enabled:   true,
	}
	if req.Password != "" {
		body.Credentials = append(body.Credentials, struct {
			Type      string `json:"type"`
			Value     string `json:"value"`
			Temporary bool   `json:"temporary"`
		}{Type: "password", Value: req.Password, Temporary: false})
	}

	path := fmt.Sprintf("/admin/realms/%s/users", tenantID)
	resp, err := c.adminRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return User{}, ErrAlreadyExists
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return User{}, fmt.Errorf("keycloak: create user %d: %s", resp.StatusCode, string(b))
	}

	// Extract user ID from Location header.
	location := resp.Header.Get("Location")
	if location == "" {
		return User{}, fmt.Errorf("keycloak: missing Location header after user creation")
	}
	parsed, err := url.Parse(location)
	if err != nil {
		return User{}, fmt.Errorf("keycloak: parse location: %w", err)
	}
	segments := splitPath(parsed.Path)
	userID := segments[len(segments)-1]

	created, err := c.GetUser(ctx, tenantID, userID)
	if err != nil {
		return User{}, err
	}

	// Assign initial roles.
	for _, role := range req.Roles {
		_ = c.AssignRole(ctx, tenantID, userID, role)
	}

	return created, nil
}

func (c *keycloakClient) UpdateUser(ctx context.Context, tenantID string, userID string, req UpdateUserRequest) (User, error) {
	current, err := c.GetUser(ctx, tenantID, userID)
	if err != nil {
		return User{}, err
	}

	patch := map[string]any{}
	if req.Email != nil {
		patch["email"] = *req.Email
	}
	if req.FirstName != nil {
		patch["firstName"] = *req.FirstName
	}
	if req.LastName != nil {
		patch["lastName"] = *req.LastName
	}
	if req.Enabled != nil {
		patch["enabled"] = *req.Enabled
	}

	path := fmt.Sprintf("/admin/realms/%s/users/%s", tenantID, userID)
	resp, err := c.adminRequest(ctx, http.MethodPut, path, patch)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return User{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return User{}, fmt.Errorf("keycloak: update user %d: %s", resp.StatusCode, string(b))
	}
	_ = current // suppress unused warning
	return c.GetUser(ctx, tenantID, userID)
}

func (c *keycloakClient) DeleteUser(ctx context.Context, tenantID string, userID string) error {
	path := fmt.Sprintf("/admin/realms/%s/users/%s", tenantID, userID)
	resp, err := c.adminRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: delete user %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *keycloakClient) AssignRole(ctx context.Context, tenantID string, userID string, role string) error {
	return c.manageRole(ctx, tenantID, userID, role, http.MethodPost)
}

func (c *keycloakClient) RemoveRole(ctx context.Context, tenantID string, userID string, role string) error {
	return c.manageRole(ctx, tenantID, userID, role, http.MethodDelete)
}

func (c *keycloakClient) manageRole(ctx context.Context, tenantID, userID, role, method string) error {
	clientUUID, err := c.getClientUUID(ctx, tenantID)
	if err != nil {
		return err
	}
	// First, resolve the role representation from Keycloak.
	rolePath := fmt.Sprintf("/admin/realms/%s/clients/%s/roles/%s", tenantID, clientUUID, role)
	roleResp, err := c.adminRequest(ctx, http.MethodGet, rolePath, nil)
	if err != nil {
		return err
	}
	defer roleResp.Body.Close()
	roleBody, _ := io.ReadAll(roleResp.Body)
	if roleResp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("keycloak: role %q not found", role)
	}
	if roleResp.StatusCode != http.StatusOK {
		return fmt.Errorf("keycloak: get role %d: %s", roleResp.StatusCode, string(roleBody))
	}
	var roleRep map[string]any
	if err := json.Unmarshal(roleBody, &roleRep); err != nil {
		return fmt.Errorf("keycloak: parse role: %w", err)
	}

	path := fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/clients/%s", tenantID, userID, clientUUID)
	resp, err := c.adminRequest(ctx, method, path, []map[string]any{roleRep})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: manage role %s %d: %s", method, resp.StatusCode, string(b))
	}
	return nil
}

func splitPath(p string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '/' {
			if seg := p[start:i]; seg != "" {
				parts = append(parts, seg)
			}
			start = i + 1
		}
	}
	return parts
}
