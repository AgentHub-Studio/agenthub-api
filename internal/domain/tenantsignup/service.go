// Package tenantsignup exposes a self-service signup endpoint that a public
// visitor can call to create their own tenant in one step: tenant row +
// Keycloak realm + schema migrations + admin user with a temp password.
//
// This is a thin orchestrator over tenant.Service and user.KeycloakClient;
// no new domain state lives here.
package tenantsignup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
)

// ErrWeakTenantID is returned when the supplied tenant slug fails validation.
var ErrWeakTenantID = errors.New("tenant id must be kebab-case (lowercase, digits, hyphens)")

// tenantCreator is the narrow slice of tenant.Service we need.
type tenantCreator interface {
	Create(ctx context.Context, req tenant.CreateTenantRequest) (tenant.TenantResponse, error)
}

// userCreator is the narrow slice of user.KeycloakClient we need.
type userCreator interface {
	CreateUser(ctx context.Context, tenantID string, req user.CreateUserRequest) (user.User, error)
}

// Service orchestrates the signup flow.
type Service struct {
	tenants         tenantCreator
	users           userCreator
	keycloakBaseURL string
	frontendBaseURL string
}

// NewService wires the signup Service.
// keycloakBaseURL is used to build the login URL returned to the user.
// frontendBaseURL (optional) lets us add a post-login redirect target.
func NewService(tenants tenantCreator, users userCreator, keycloakBaseURL, frontendBaseURL string) *Service {
	return &Service{
		tenants:         tenants,
		users:           users,
		keycloakBaseURL: strings.TrimRight(keycloakBaseURL, "/"),
		frontendBaseURL: strings.TrimRight(frontendBaseURL, "/"),
	}
}

// Signup creates the tenant and its initial admin user. The caller is
// expected to guard this endpoint (rate limit / captcha) at the edge.
func (s *Service) Signup(ctx context.Context, req SignupRequest) (SignupResponse, error) {
	tenantID := strings.ToLower(strings.TrimSpace(req.TenantID))
	if tenantID == "" || !isKebab(tenantID) {
		return SignupResponse{}, ErrWeakTenantID
	}
	if strings.TrimSpace(req.TenantName) == "" {
		return SignupResponse{}, fmt.Errorf("tenantName is required")
	}
	if _, err := mail.ParseAddress(req.AdminEmail); err != nil {
		return SignupResponse{}, fmt.Errorf("adminEmail is invalid")
	}
	if strings.TrimSpace(req.AdminFirstName) == "" {
		return SignupResponse{}, fmt.Errorf("adminFirstName is required")
	}

	// 1. Create tenant (Keycloak realm + schema migration + LLM preset seed).
	if _, err := s.tenants.Create(ctx, tenant.CreateTenantRequest{
		ID:   tenantID,
		Name: strings.TrimSpace(req.TenantName),
	}); err != nil {
		return SignupResponse{}, fmt.Errorf("tenantsignup: create tenant: %w", err)
	}

	// 2. Generate temp password and create the admin user.
	tempPwd, err := randomPassword(20)
	if err != nil {
		return SignupResponse{}, fmt.Errorf("tenantsignup: generate password: %w", err)
	}
	username := usernameFromEmail(req.AdminEmail)
	if _, err := s.users.CreateUser(ctx, tenantID, user.CreateUserRequest{
		Username:  username,
		Email:     req.AdminEmail,
		FirstName: req.AdminFirstName,
		LastName:  req.AdminLastName,
		Password:  tempPwd,
		Roles:     []string{"admin"},
	}); err != nil {
		return SignupResponse{}, fmt.Errorf("tenantsignup: create admin user: %w", err)
	}

	return SignupResponse{
		TenantID:     tenantID,
		Username:     username,
		LoginURL:     s.buildLoginURL(tenantID),
		TempPassword: tempPwd,
	}, nil
}

func (s *Service) buildLoginURL(tenantID string) string {
	if s.keycloakBaseURL == "" {
		return ""
	}
	base := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/auth", s.keycloakBaseURL, tenantID)
	if s.frontendBaseURL == "" {
		return base
	}
	return fmt.Sprintf("%s?redirect_uri=%s", base, s.frontendBaseURL)
}

// isKebab accepts lowercase letters, digits, and hyphens (no leading/trailing hyphen).
// Mirrors the regex in tenant.Service for consistency — kept local to avoid a circular
// dependency on an internal-only symbol.
func isKebab(s string) bool {
	if len(s) < 2 || len(s) > 63 {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
			continue
		default:
			return false
		}
	}
	return true
}

// usernameFromEmail returns the local part of an email address (portion before '@').
// The full email becomes the Keycloak email attribute; username is the local part
// to keep it short and memorable.
func usernameFromEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	return email[:at]
}

// randomPassword returns a base64-encoded random string of about n characters.
// 20 bytes of entropy → 27 base64 chars, trimmed to n.
func randomPassword(n int) (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	s := base64.RawURLEncoding.EncodeToString(buf)
	if len(s) > n {
		s = s[:n]
	}
	return s, nil
}
