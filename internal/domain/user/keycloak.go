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
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when the user cannot be found.
var ErrNotFound = errors.New("user: not found")

// ErrAlreadyExists is returned when the username/email is already taken.
var ErrAlreadyExists = errors.New("user: already exists")

// ErrValidation indicates a request payload failed validation.
// Maps to HTTP 422 in handler.
var ErrValidation = errors.New("user: validation failed")

// KeycloakUserClient defines the Keycloak Admin API operations for users.
type KeycloakUserClient interface {
	ListUsers(ctx context.Context, tenantID string) ([]User, error)
	GetUser(ctx context.Context, tenantID string, userID string) (User, error)
	CreateUser(ctx context.Context, tenantID string, req CreateUserRequest) (User, error)
	UpdateUser(ctx context.Context, tenantID string, userID string, req UpdateUserRequest) (User, error)
	DeleteUser(ctx context.Context, tenantID string, userID string) error
	AssignRole(ctx context.Context, tenantID string, userID string, role string) error
	RemoveRole(ctx context.Context, tenantID string, userID string, role string) error
	ResetPassword(ctx context.Context, tenantID string, userID string) error
	ListRoles(ctx context.Context, tenantID string) ([]string, error)
}

// keycloakClient is the HTTP implementation of KeycloakUserClient.
type keycloakClient struct {
	baseURL        string
	adminUsername  string
	adminPassword  string
	adminClientID  string
	adminRealm     string
	httpClient     *http.Client
	tokenMu        sync.Mutex
	cachedToken    string
	tokenExpiresAt time.Time
	// Bug 271: cache de roles in-memory para mitigar bug 267
	// (Keycloak admin API lento em cluster k3s). TTL 60s mantém
	// staleness baixa; roles mudam raramente. Key: tenantID:userID.
	rolesMu    sync.RWMutex
	rolesCache map[string]rolesCacheEntry
}

type rolesCacheEntry struct {
	roles     []string
	expiresAt time.Time
}

// KeycloakClientConfig holds configuration for the Keycloak Admin API client.
type KeycloakClientConfig struct {
	BaseURL        string // e.g. http://keycloak.internal:8080
	AdminUsername  string
	AdminPassword  string
	AdminClientID  string // defaults to admin-cli
	AdminRealm     string // defaults to master
	FrontendClient string // deprecated: user roles are tenant realm roles
}

// NewKeycloakUserClient creates a new Keycloak Admin API user client.
func NewKeycloakUserClient(cfg KeycloakClientConfig) KeycloakUserClient {
	if cfg.AdminClientID == "" {
		cfg.AdminClientID = "admin-cli"
	}
	if cfg.AdminRealm == "" {
		cfg.AdminRealm = "master"
	}
	return &keycloakClient{
		baseURL:       cfg.BaseURL,
		adminUsername: cfg.AdminUsername,
		adminPassword: cfg.AdminPassword,
		adminClientID: cfg.AdminClientID,
		adminRealm:    cfg.AdminRealm,
		// Bug 269: Keycloak admin API às vezes leva >15s para responder
		// (observado em GET /admin/realms/X/users em cluster k3s). 60s
		// dá margem para Keycloak pod sob carga sem aborta requests
		// legítimos. Logs mostravam: context deadline exceeded em 15s
		// para chamadas que de fato respondem em 25-30s.
		httpClient: &http.Client{Timeout: 60 * time.Second},
		rolesCache: make(map[string]rolesCacheEntry),
	}
}

// getAdminToken obtains a short-lived admin access token from the master realm.
// Caches the token in-memory until ~5s before expiry to avoid re-login per request
// (P-C292: GET /api/users took 24s because each Admin API call triggered a fresh
// password grant; 60s token TTL × N+1 user role lookups compounded the latency).
func (c *keycloakClient) getAdminToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.cachedToken != "" && time.Now().Before(c.tokenExpiresAt) {
		return c.cachedToken, nil
	}

	tokenURL, err := buildKeycloakURL(c.baseURL, "realms", c.adminRealm, "protocol", "openid-connect", "token")
	if err != nil {
		return "", err
	}
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", c.adminClientID)
	data.Set("username", c.adminUsername)
	data.Set("password", c.adminPassword)

	// #nosec G704 -- tokenURL is built from a trusted Keycloak base URL and escaped path segments.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("keycloak: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// #nosec G704 -- req URL is built by buildKeycloakURL from a trusted base URL and escaped path segments.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak: token response %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("keycloak: parse token: %w", err)
	}
	c.cachedToken = result.AccessToken
	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= 5*time.Second {
		ttl = 60 * time.Second
	}
	c.tokenExpiresAt = time.Now().Add(ttl - 5*time.Second)
	return c.cachedToken, nil
}

func (c *keycloakClient) adminRequest(ctx context.Context, method, tenantID string, bodyV any, segments ...string) (*http.Response, error) {
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
	fullURL, err := buildKeycloakURL(c.baseURL, append([]string{"admin", "realms", tenantID}, segments...)...)
	if err != nil {
		return nil, err
	}
	// #nosec G704 -- fullURL is built from a trusted Keycloak base URL and escaped path segments.
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("keycloak: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if bodyV != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// #nosec G704 -- req URL is built by buildKeycloakURL from a trusted base URL and escaped path segments.
	return c.httpClient.Do(req)
}

func buildKeycloakURL(rawBase string, segments ...string) (string, error) {
	parsed, err := parseKeycloakBaseURL(rawBase)
	if err != nil {
		return "", err
	}
	escapedPath := strings.TrimSuffix(parsed.EscapedPath(), "/")
	for _, segment := range segments {
		if segment == "" || containsKeycloakURLControlChar(segment) {
			return "", fmt.Errorf("keycloak: invalid URL path segment")
		}
		escapedPath += "/" + url.PathEscape(segment)
	}
	if escapedPath == "" {
		escapedPath = "/"
	}
	unescapedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", err
	}
	next := *parsed
	next.Path = unescapedPath
	next.RawPath = escapedPath
	return next.String(), nil
}

func parseKeycloakBaseURL(rawBase string) (*url.URL, error) {
	if rawBase == "" || containsKeycloakURLControlChar(rawBase) {
		return nil, fmt.Errorf("keycloak: invalid base URL: empty or unsafe")
	}
	parsed, err := url.Parse(strings.TrimRight(rawBase, "/"))
	if err != nil {
		return nil, fmt.Errorf("keycloak: invalid base URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("keycloak: invalid base URL: unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("keycloak: invalid base URL: missing host")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("keycloak: invalid base URL: userinfo, query and fragment are not allowed")
	}
	return parsed, nil
}

func containsKeycloakURLControlChar(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
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

// rolesCacheTTL is how long a roles entry stays cached. Bug 271: roles
// mudam raramente; 60s reduz drasticamente o N+1 em GET /api/users.
const rolesCacheTTL = 60 * time.Second

func (c *keycloakClient) rolesCacheKey(tenantID, userID string) string {
	return tenantID + ":" + userID
}

// rolesFromCache retrieves cached roles if not expired; otherwise returns nil, false.
func (c *keycloakClient) rolesFromCache(tenantID, userID string) ([]string, bool) {
	key := c.rolesCacheKey(tenantID, userID)
	c.rolesMu.RLock()
	defer c.rolesMu.RUnlock()
	entry, ok := c.rolesCache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return append([]string(nil), entry.roles...), true
}

// rolesCacheMaxEntries é o threshold a partir do qual rolesCacheStore faz
// sweep de entries expiradas. Bug 272: sem GC periódico, entries expiradas
// (após TTL 60s) ficavam no map indefinidamente. Para 1000 users × 100
// tenants × 50B = 5MB. Sweep amortizado mantém footprint limitado.
const rolesCacheMaxEntries = 1000

// rolesCacheStore writes roles to cache with current TTL. Faz sweep de
// entries expiradas se o map exceder rolesCacheMaxEntries.
func (c *keycloakClient) rolesCacheStore(tenantID, userID string, roles []string) {
	key := c.rolesCacheKey(tenantID, userID)
	c.rolesMu.Lock()
	defer c.rolesMu.Unlock()
	if len(c.rolesCache) >= rolesCacheMaxEntries {
		now := time.Now()
		for k, e := range c.rolesCache {
			if now.After(e.expiresAt) {
				delete(c.rolesCache, k)
			}
		}
	}
	c.rolesCache[key] = rolesCacheEntry{
		roles:     append([]string(nil), roles...),
		expiresAt: time.Now().Add(rolesCacheTTL),
	}
}

// rolesCacheInvalidate evicts the entry for (tenantID, userID). Called
// after AssignRole/RemoveRole/DeleteUser to prevent stale data.
func (c *keycloakClient) rolesCacheInvalidate(tenantID, userID string) {
	key := c.rolesCacheKey(tenantID, userID)
	c.rolesMu.Lock()
	delete(c.rolesCache, key)
	c.rolesMu.Unlock()
}

func (c *keycloakClient) getUserRoles(ctx context.Context, tenantID, userID string) ([]string, error) {
	if cached, ok := c.rolesFromCache(tenantID, userID); ok {
		return cached, nil
	}
	resp, err := c.adminRequest(ctx, http.MethodGet, tenantID, nil, "users", userID, "role-mappings", "realm")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		c.rolesCacheStore(tenantID, userID, []string{})
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
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		if role.Name != "" {
			names = append(names, role.Name)
		}
	}
	c.rolesCacheStore(tenantID, userID, names)
	return names, nil
}

func (c *keycloakClient) ListUsers(ctx context.Context, tenantID string) ([]User, error) {
	resp, err := c.adminRequest(ctx, http.MethodGet, tenantID, nil, "users")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak: list users %d: %s", resp.StatusCode, string(body))
	}
	var kcUsers []kcUser
	if err := json.Unmarshal(body, &kcUsers); err != nil {
		return nil, fmt.Errorf("keycloak: parse users: %w", err)
	}
	users := make([]User, len(kcUsers))
	var (
		wg        sync.WaitGroup
		roleErr   error
		roleErrMu sync.Mutex
	)
	for i, ku := range kcUsers {
		wg.Add(1)
		go func(i int, ku kcUser) {
			defer wg.Done()
			roles, err := c.getUserRoles(ctx, tenantID, ku.ID)
			if err != nil {
				roleErrMu.Lock()
				if roleErr == nil {
					roleErr = fmt.Errorf("keycloak: list user roles for %q: %w", ku.ID, err)
				}
				roleErrMu.Unlock()
				return
			}
			users[i] = kcUserToUser(ku, roles)
		}(i, ku)
	}
	wg.Wait()
	if roleErr != nil {
		return nil, roleErr
	}
	return users, nil
}

func (c *keycloakClient) GetUser(ctx context.Context, tenantID string, userID string) (User, error) {
	resp, err := c.adminRequest(ctx, http.MethodGet, tenantID, nil, "users", userID)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	roles, err := c.getUserRoles(ctx, tenantID, ku.ID)
	if err != nil {
		return User{}, err
	}
	return kcUserToUser(ku, roles), nil
}

func (c *keycloakClient) CreateUser(ctx context.Context, tenantID string, req CreateUserRequest) (User, error) {
	type kcCreateReq struct {
		Username        string   `json:"username"`
		Email           string   `json:"email"`
		FirstName       string   `json:"firstName"`
		LastName        string   `json:"lastName"`
		Enabled         bool     `json:"enabled"`
		EmailVerified   bool     `json:"emailVerified"`
		RequiredActions []string `json:"requiredActions"`
		Credentials     []struct {
			Type      string `json:"type"`
			Value     string `json:"value"`
			Temporary bool   `json:"temporary"`
		} `json:"credentials"`
	}
	firstName := strings.TrimSpace(req.FirstName)
	if firstName == "" {
		firstName = strings.TrimSpace(req.Username)
	}
	lastName := strings.TrimSpace(req.LastName)
	if lastName == "" {
		lastName = strings.TrimSpace(req.Username)
	}
	body := kcCreateReq{
		Username:        req.Username,
		Email:           req.Email,
		FirstName:       firstName,
		LastName:        lastName,
		Enabled:         true,
		EmailVerified:   true,
		RequiredActions: []string{},
	}
	if req.Password != "" {
		body.Credentials = append(body.Credentials, struct {
			Type      string `json:"type"`
			Value     string `json:"value"`
			Temporary bool   `json:"temporary"`
		}{Type: "password", Value: req.Password, Temporary: false})
	}

	resp, err := c.adminRequest(ctx, http.MethodPost, tenantID, body, "users")
	if err != nil {
		return User{}, err
	}
	defer func() { _ = resp.Body.Close() }()

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

	resp, err := c.adminRequest(ctx, http.MethodPut, tenantID, patch, "users", userID)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = resp.Body.Close() }()
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
	resp, err := c.adminRequest(ctx, http.MethodDelete, tenantID, nil, "users", userID)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: delete user %d: %s", resp.StatusCode, string(b))
	}
	c.rolesCacheInvalidate(tenantID, userID)
	return nil
}

func (c *keycloakClient) AssignRole(ctx context.Context, tenantID string, userID string, role string) error {
	err := c.manageRole(ctx, tenantID, userID, role, http.MethodPost)
	if err == nil {
		c.rolesCacheInvalidate(tenantID, userID)
	}
	return err
}

func (c *keycloakClient) RemoveRole(ctx context.Context, tenantID string, userID string, role string) error {
	err := c.manageRole(ctx, tenantID, userID, role, http.MethodDelete)
	if err == nil {
		c.rolesCacheInvalidate(tenantID, userID)
	}
	return err
}

func (c *keycloakClient) manageRole(ctx context.Context, tenantID, userID, role, method string) error {
	// First, resolve the role representation from Keycloak.
	roleResp, err := c.adminRequest(ctx, http.MethodGet, tenantID, nil, "roles", role)
	if err != nil {
		return err
	}
	defer func() { _ = roleResp.Body.Close() }()
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

	resp, err := c.adminRequest(ctx, method, tenantID, []map[string]any{roleRep}, "users", userID, "role-mappings", "realm")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: manage role %s %d: %s", method, resp.StatusCode, string(b))
	}
	return nil
}

// ResetPassword triggers a Keycloak "UPDATE_PASSWORD" required action, sending a reset email.
func (c *keycloakClient) ResetPassword(ctx context.Context, tenantID string, userID string) error {
	resp, err := c.adminRequest(ctx, http.MethodPut, tenantID, []string{"UPDATE_PASSWORD"}, "users", userID, "execute-actions-email")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak: reset password %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ListRoles returns tenant realm roles managed by AgentHub.
func (c *keycloakClient) ListRoles(ctx context.Context, tenantID string) ([]string, error) {
	resp, err := c.adminRequest(ctx, http.MethodGet, tenantID, nil, "roles")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak: list roles %d: %s", resp.StatusCode, string(body))
	}
	var roles []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, fmt.Errorf("keycloak: parse roles: %w", err)
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		if r.Name == "" || isKeycloakDefaultRealmRole(tenantID, r.Name) {
			continue
		}
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return names, nil
}

func isKeycloakDefaultRealmRole(tenantID, role string) bool {
	return role == "offline_access" ||
		role == "uma_authorization" ||
		role == "default-roles-"+tenantID
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
