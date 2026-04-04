package auth

// Config holds the JWT middleware configuration.
type Config struct {
	KeycloakBaseURL string // e.g. "https://keycloak.example.com"
	// AllowedIssuers is optional; if empty any issuer under KeycloakBaseURL is accepted.
	AllowedIssuers []string
}
