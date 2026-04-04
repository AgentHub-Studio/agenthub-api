// Package keycloak provides a Keycloak Admin API client for AgentHub Go services.
// It handles realm provisioning, user CRUD, role assignment, and client management
// using the master realm admin-cli credentials.
//
// Usage:
//
//	client := keycloak.NewAdminClient(keycloak.Config{
//	    BaseURL:       cfg.KeycloakBaseURL,
//	    AdminUsername: cfg.KeycloakAdminUsername,
//	    AdminPassword: cfg.KeycloakAdminPassword,
//	})
//	err := client.CreateRealm(ctx, tenantID)
package keycloak
