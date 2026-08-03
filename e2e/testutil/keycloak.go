//go:build e2e

package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// KeycloakClient wraps the Keycloak Admin REST API and token endpoint.
type KeycloakClient struct {
	BaseURL       string
	AdminUser     string
	AdminPassword string
	http          *http.Client
}

// NewKeycloakClient creates a KeycloakClient using env-supplied config.
func NewKeycloakClient(baseURL, adminUser, adminPassword string) *KeycloakClient {
	return &KeycloakClient{
		BaseURL:       baseURL,
		AdminUser:     adminUser,
		AdminPassword: adminPassword,
		http:          &http.Client{Timeout: 30 * time.Second},
	}
}

// AdminToken obtains a short-lived token from the master realm admin-cli client.
func (k *KeycloakClient) AdminToken() (string, error) {
	return k.fetchToken("master", "admin-cli", k.AdminUser, k.AdminPassword)
}

// UserToken obtains a JWT for the given realm / user via ROPC.
func (k *KeycloakClient) UserToken(realm, username, password string) (string, error) {
	return k.fetchToken(realm, "agenthub-frontend", username, password)
}

// CreateAdminUser creates a user in the given realm, sets a permanent password
// and assigns the "admin" realm role. Returns the new user's UUID.
func (k *KeycloakClient) CreateAdminUser(realm, username, password string) (string, error) {
	adminToken, err := k.AdminToken()
	if err != nil {
		return "", fmt.Errorf("keycloak: admin token: %w", err)
	}

	// 1. Create user
	body, _ := json.Marshal(map[string]any{
		"username":      username,
		"enabled":       true,
		"emailVerified": true,
		"email":         username + "@e2e.test",
		"credentials": []map[string]any{
			{"type": "password", "value": password, "temporary": false},
		},
	})
	resp, err := k.adminRequest(http.MethodPost,
		fmt.Sprintf("/admin/realms/%s/users", realm),
		adminToken, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("keycloak: create user: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: create user %d: %s", resp.StatusCode, raw)
	}

	location := resp.Header.Get("Location")
	userID := location[strings.LastIndex(location, "/")+1:]

	// 2. Fetch "admin" role
	roleResp, err := k.adminRequest(http.MethodGet,
		fmt.Sprintf("/admin/realms/%s/roles/admin", realm),
		adminToken, nil)
	if err != nil {
		return userID, fmt.Errorf("keycloak: get admin role: %w", err)
	}
	defer func() { _ = roleResp.Body.Close() }()
	rawRole, _ := io.ReadAll(roleResp.Body)

	// 3. Assign role
	assignResp, err := k.adminRequest(http.MethodPost,
		fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/realm", realm, userID),
		adminToken, bytes.NewReader([]byte("["+string(rawRole)+"]")))
	if err != nil {
		return userID, fmt.Errorf("keycloak: assign role: %w", err)
	}
	defer func() { _ = assignResp.Body.Close() }()
	return userID, nil
}

// DeleteRealm deletes the given realm. Errors are logged but not fatal.
func (k *KeycloakClient) DeleteRealm(realm string) error {
	adminToken, err := k.AdminToken()
	if err != nil {
		return err
	}
	resp, err := k.adminRequest(http.MethodDelete,
		fmt.Sprintf("/admin/realms/%s", realm),
		adminToken, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// WaitForRealm polls until the realm is accessible or maxWait is exceeded.
func (k *KeycloakClient) WaitForRealm(realm string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		resp, err := k.http.Get(k.BaseURL + "/realms/" + realm)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("keycloak: realm %q not ready after %s", realm, maxWait)
}

func (k *KeycloakClient) fetchToken(realm, clientID, username, password string) (string, error) {
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {username},
		"password":   {password},
	}
	resp, err := k.http.Post(
		k.BaseURL+"/realms/"+realm+"/protocol/openid-connect/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("keycloak: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: token %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("keycloak: decode token: %w", err)
	}
	return result.AccessToken, nil
}

func (k *KeycloakClient) adminRequest(method, path, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, k.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return k.http.Do(req)
}
