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

// Config holds Keycloak Admin API configuration.
type Config struct {
	BaseURL       string `env:"KEYCLOAK_BASE_URL,required"`
	AdminUsername string `env:"KEYCLOAK_ADMIN_USERNAME,required"`
	AdminPassword string `env:"KEYCLOAK_ADMIN_PASSWORD,required"`
	AdminRealm    string `env:"KEYCLOAK_ADMIN_REALM"    envDefault:"master"`
	AdminClientID string `env:"KEYCLOAK_ADMIN_CLIENT_ID" envDefault:"admin-cli"`
	ClientID      string `env:"KEYCLOAK_CLIENT_ID"       envDefault:"agenthub-frontend"`
}

// AdminClient is a client for the Keycloak Admin REST API.
type AdminClient struct {
	cfg        Config
	httpClient *http.Client
}

// NewAdminClient creates a new AdminClient.
func NewAdminClient(cfg Config) *AdminClient {
	return &AdminClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// getAdminToken obtains an admin access token from the master realm.
func (c *AdminClient) getAdminToken(ctx context.Context) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.cfg.BaseURL, c.cfg.AdminRealm)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", c.cfg.AdminClientID)
	form.Set("username", c.cfg.AdminUsername)
	form.Set("password", c.cfg.AdminPassword)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("keycloak: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: get admin token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: admin token status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("keycloak: decode token response: %w", err)
	}
	return result.AccessToken, nil
}

func (c *AdminClient) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	token, err := c.getAdminToken(ctx)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("keycloak: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	fullURL := c.cfg.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("keycloak: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}
