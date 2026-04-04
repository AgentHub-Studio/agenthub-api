package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// User represents a Keycloak user.
type User struct {
	ID        string `json:"id,omitempty"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Enabled   bool   `json:"enabled"`
}

// CreateUser creates a user in the given realm.
func (c *AdminClient) CreateUser(ctx context.Context, tenantID string, user User) (string, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/admin/realms/%s/users", tenantID), user)
	if err != nil {
		return "", fmt.Errorf("keycloak: create user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak: create user status %d: %s", resp.StatusCode, b)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("keycloak: create user: missing Location header")
	}
	// Extract ID from Location: .../users/{id}
	parts := splitLast(loc, "/")
	return parts, nil
}

// GetUser retrieves a user by ID.
func (c *AdminClient) GetUser(ctx context.Context, tenantID, userID string) (*User, error) {
	resp, err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/admin/realms/%s/users/%s", tenantID, userID), nil)
	if err != nil {
		return nil, fmt.Errorf("keycloak: get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("keycloak: user %q not found", userID)
	}

	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("keycloak: decode user: %w", err)
	}
	return &u, nil
}

// DeleteUser deletes a user by ID.
func (c *AdminClient) DeleteUser(ctx context.Context, tenantID, userID string) error {
	resp, err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/admin/realms/%s/users/%s", tenantID, userID), nil)
	if err != nil {
		return fmt.Errorf("keycloak: delete user: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("keycloak: delete user status %d", resp.StatusCode)
	}
	return nil
}

func splitLast(s, sep string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == sep[0] {
			return s[i+1:]
		}
	}
	return s
}
